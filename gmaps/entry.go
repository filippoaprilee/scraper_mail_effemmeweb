package gmaps

import (
	"encoding/json"
	"sync"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
	"net"
	"strconv"
	"os"
    "context"
	
	"github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/proto"
    "github.com/domainr/whois"
    "github.com/ns3777k/go-shodan/v4/shodan"
)
// "github.com/PuerkitoBio/goquery"
// "github.com/domainr/whois"

type Address struct {
	Borough    string `json:"borough"`
	Street     string `json:"street"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	State      string `json:"state"`
	Country    string `json:"country"`
}

type Entry struct {
	ID           string `json:"id"`
	Link         string `json:"link"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	WebSite      string `json:"web_site"`
	Protocol     string `json:"protocol"`
	Technology   string `json:"technology"`
	Phone        string `json:"phone"`
	Street       string `json:"street"`
	City         string `json:"city"`
	Province     string `json:"province"`
	Email        string `json:"email"`
	CookieBanner string `json:"cookie_banner"`
	HostingProvider string `json:"hosting_provider"` 
    MobilePerformance string  `json:"mobile_performance"`  // Cambiato a string
    DesktopPerformance string `json:"desktop_performance"` // Cambiato a string
    SeoScore          string  `json:"seo_score"`           // Cambiato a string
    SiteAvailability    string `json:"site_availability"`   // Nuovo campo
    SiteMaintenance     string `json:"site_maintenance"`    // Nuovo campo
}

type PagespeedResponse struct {
	LighthouseResult struct {
		Categories struct {
			Performance struct {
				Score float64 `json:"score"`
			} `json:"performance"`
			SEO struct {
				Score float64 `json:"score"`
			} `json:"seo"`
		} `json:"categories"`
	} `json:"lighthouseResult"`
}

func loadCmsNames(filename string) (map[string][]string, error) {
	cmsMap := make(map[string][]string)
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("errore nell'aprire il file %s: %v", filename, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cmsMap)
	if err != nil {
		return nil, fmt.Errorf("errore nella decodifica del file JSON %s: %v", filename, err)
	}
	return cmsMap, nil
}

func loadExcludedWebsites(filename string) (map[string]struct{}, error) {
    // Struttura per il parsing del JSON
    var data struct {
        ExcludedDomains []string `json:"excluded_domains"`
    }

    // Apri il file JSON
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("errore nell'aprire il file %s: %v", filename, err)
    }
    defer file.Close()

    // Decodifica il file JSON
    decoder := json.NewDecoder(file)
    if err := decoder.Decode(&data); err != nil {
        return nil, fmt.Errorf("errore nel parsing del file JSON: %v", err)
    }

    // Popola la mappa con i domini esclusi
    excluded := make(map[string]struct{})
    for _, domain := range data.ExcludedDomains {
        excluded[domain] = struct{}{}
    }

    return excluded, nil
}

// Funzione per ottenere i punteggi SEO, Mobile e Desktop
func getPageSpeedScores(url string) (int, int, float64, error) {
    apiKey := "AIzaSyD13bhKEEwzY15yMgsolkVvMCuToZsHPlU" // Inserisci la tua API Key qui
    maxAttempts := 3    // Numero massimo di tentativi per ogni richiesta
    delay := 2 * time.Second

    // URL per ottenere i punteggi SEO
    seoURL := fmt.Sprintf("https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=desktop&category=seo&key=%s", url, apiKey)
    
    // URL per ottenere i punteggi di performance (mobile e desktop)
    mobileURL := fmt.Sprintf("https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=mobile&key=%s", url, apiKey)
    desktopURL := fmt.Sprintf("https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=desktop&key=%s", url, apiKey)

    var seoData, mobileData, desktopData PagespeedResponse
    var errSeo, errMobile, errDesktop error
    var wg sync.WaitGroup

    // Funzione per eseguire richieste con tentativi ripetuti
    requestWithRetries := func(url string, data *PagespeedResponse, err *error) {
        for attempt := 1; attempt <= maxAttempts; attempt++ {
            resp, reqErr := http.Get(url)
            if reqErr != nil {
                *err = fmt.Errorf("errore tentativo %d: %v", attempt, reqErr)
                time.Sleep(delay)
                continue
            }
            defer resp.Body.Close()

            if resp.StatusCode == http.StatusTooManyRequests { // Gestione limite API
                retryAfter := resp.Header.Get("Retry-After")
                if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
                    time.Sleep(time.Duration(seconds) * time.Second)
                } else {
                    time.Sleep(delay)
                }
                *err = fmt.Errorf("limite API raggiunto, ritentando...")
                continue
            }

            body, readErr := io.ReadAll(resp.Body)
            if readErr != nil {
                *err = fmt.Errorf("errore nella lettura del corpo della risposta: %v", readErr)
                time.Sleep(delay)
                continue
            }

            if jsonErr := json.Unmarshal(body, data); jsonErr != nil {
                *err = fmt.Errorf("errore nel parsing JSON: %v", jsonErr)
                time.Sleep(delay)
                continue
            }

            *err = nil // Nessun errore
            break
        }
    }

    // Avvio richieste in parallelo con tentativi
    wg.Add(3)
    go func() {
        defer wg.Done()
        requestWithRetries(seoURL, &seoData, &errSeo)
    }()
    go func() {
        defer wg.Done()
        requestWithRetries(mobileURL, &mobileData, &errMobile)
    }()
    go func() {
        defer wg.Done()
        requestWithRetries(desktopURL, &desktopData, &errDesktop)
    }()
    wg.Wait()

    // Gestione errori finali
    if errSeo != nil {
        return 0, 0, 0, fmt.Errorf("errore nella richiesta SEO: %v", errSeo)
    }
    if errMobile != nil {
        return 0, 0, 0, fmt.Errorf("errore nella richiesta Mobile: %v", errMobile)
    }
    if errDesktop != nil {
        return 0, 0, 0, fmt.Errorf("errore nella richiesta Desktop: %v", errDesktop)
    }

    // Accedi ai punteggi di Performance e SEO
    mobilePerformance := int(mobileData.LighthouseResult.Categories.Performance.Score * 100)
    desktopPerformance := int(desktopData.LighthouseResult.Categories.Performance.Score * 100)
    seoScore := seoData.LighthouseResult.Categories.SEO.Score * 100

    return mobilePerformance, desktopPerformance, seoScore, nil
}

func loadProviderMapping(filename string) (map[string]string, map[string]*regexp.Regexp, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, nil, fmt.Errorf("Errore nell'aprire il file %s: %v", filename, err)
    }
    defer file.Close()

    providerMapping := make(map[string]string)
    decoder := json.NewDecoder(file)
    if err := decoder.Decode(&providerMapping); err != nil {
        return nil, nil, fmt.Errorf("Errore nella decodifica del file JSON %s: %v", filename, err)
    }

    // Compila i wildcard per uso successivo
    compiledMapping := make(map[string]*regexp.Regexp)
    for pattern := range providerMapping {
        regexPattern := strings.ReplaceAll(pattern, "*", ".*")
        compiledMapping[pattern] = regexp.MustCompile("^" + regexPattern + "$")
    }

    return providerMapping, compiledMapping, nil
}

// Usa WHOIS per ottenere informazioni sul dominio
func getHostingProviderFromWhois(domain string) (string, error) {
    req, err := whois.NewRequest(domain)
    if err != nil {
        return "Sconosciuto", fmt.Errorf("errore WHOIS: %v", err)
    }

    resp, err := whois.DefaultClient.Fetch(req)
    if err != nil {
        return "Sconosciuto", fmt.Errorf("errore durante richiesta WHOIS: %v", err)
    }

    data := resp.String()
    commonProviders := map[string]string{
        "Cloudflare": "Cloudflare",
        "Amazon":     "Amazon Web Services",
        "Microsoft":  "Microsoft Azure",
        "Google":     "Google Cloud",
        "Aruba":      "Aruba Hosting",
    }

    for key, provider := range commonProviders {
        if strings.Contains(data, key) {
            // Rimuovi virgolette doppie
            return strings.ReplaceAll(provider, "\"", ""), nil
        }
    }

    return "Sconosciuto", nil
}

func getHostingProviderWithMultipleNameservers(domain, providerFile string) (string, error) {
    parsedDomain, err := estraiDominio(domain)
    if err != nil {
        return "Sconosciuto", fmt.Errorf("errore durante l'estrazione del dominio: %v", err)
    }

    nameservers, err := net.LookupNS(parsedDomain)
    if err != nil {
        // Fallback a WHOIS
        return getHostingProviderFromWhois(parsedDomain)
    }

    var providers []string
    for _, ns := range nameservers {
        normalizedNS := normalizeNameserver(ns.Host)
        hostingProvider, err := identificaHostingDaNameserver(normalizedNS, parsedDomain, providerFile) // Passa anche il dominio
        if err == nil && hostingProvider != "Sconosciuto" {
            providers = append(providers, hostingProvider)
        } else {
            // Loggare nameserver non riconosciuti insieme al dominio
            logUnknownNameserver(normalizedNS, parsedDomain)
        }
    }

    if len(providers) > 0 {
        // Unisci i provider trovati
        return strings.Join(providers, ", "), nil
    }

    // Fallback a WHOIS se nessun nameserver corrisponde
    return getHostingProviderFromWhois(parsedDomain)
}

func getHostingProviderWithFile(domain, providerFile string) (string, error) {
	parsedDomain, err := estraiDominio(domain)
	if err != nil {
		return "Sconosciuto", fmt.Errorf("errore durante l'estrazione del dominio: %v", err)
	}

	// Risoluzione IP
	ipAddress, err := resolveIP(parsedDomain)
	if err != nil {
		ipAddress = "IP non risolto"
	}

	// Chiamata a IP-API per ottenere il provider di hosting tramite IP
	if ipAddress != "IP non risolto" {
		if provider, err := getHostingFromIPAPI(ipAddress); err == nil {
			return provider, nil
		}
	}

	// Lookup NS e verifica tramite file JSON
	nameservers, err := net.LookupNS(parsedDomain)
	if err != nil {
		return getHostingProviderFromWhois(parsedDomain)
	}

	for _, ns := range nameservers {
		normalizedNS := normalizeNameserver(ns.Host)
		hostingProvider, err := identificaHostingDaNameserver(normalizedNS, parsedDomain, providerFile)
		if err == nil && hostingProvider != "Sconosciuto" {
			return hostingProvider, nil
		}
	}

	// Fallback a WHOIS
	return getHostingProviderFromWhois(parsedDomain)
}

func resolveIP(domain string) (string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", fmt.Errorf("errore risoluzione IP: %v", err)
	}
	if len(ips) > 0 {
		return ips[0].String(), nil
	}
	return "", fmt.Errorf("IP non trovato")
}

func getHostingFromIPAPI(ip string) (string, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s", ip)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("errore richiesta IP-API: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("errore parsing JSON IP-API: %v", err)
	}

	if data["status"] == "success" {
		if provider, ok := data["isp"].(string); ok {
			return provider, nil
		}
	}

	return "Sconosciuto", nil
}


func identificaHostingDaNameserver(nameserver, domain, providerFile string) (string, error) {
    providerMapping, compiledMapping, err := loadProviderMapping(providerFile)
    if err != nil {
        return "Sconosciuto", fmt.Errorf("errore caricamento provider file: %v", err)
    }

    normalizedNS := normalizeNameserver(nameserver)
    for key, regex := range compiledMapping {
        if regex.MatchString(normalizedNS) {
            hostingProvider := providerMapping[key]
            // Rimuovi virgolette doppie
            return strings.ReplaceAll(hostingProvider, "\"", ""), nil
        }
    }

    // Fallback a Shodan se IP-API e WHOIS non trovano risultati
    ip, err := resolveIP(domain)
    if err == nil {
        if shodanProvider, err := getHostingFromShodan(ip); err == nil {
            // Rimuovi virgolette doppie
            return strings.ReplaceAll(shodanProvider, "\"", ""), nil
        }
    }

    return "Sconosciuto", nil
}

// Funzione per loggare nameserver sconosciuti per analisi futura
func logUnknownNameserver(nameserver, domain string) {
    logFile := "unknown_nameservers.log"

    file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        fmt.Printf("Errore durante l'apertura del file di log: %v\n", err)
        return
    }
    defer file.Close()

    normalizedNS := normalizeNameserver(nameserver)
    whoisProvider, _ := getHostingProviderFromWhois(nameserver)

    logMessage := fmt.Sprintf(
        "Nameserver sconosciuto: %s (dominio: %s)\n  Normalizzato: %s\n  WHOIS Provider: %s\n",
        nameserver, domain, normalizedNS, whoisProvider,
    )

    if _, err := file.WriteString(logMessage); err != nil {
        fmt.Printf("Errore durante la scrittura nel file di log: %v\n", err)
    }
}

func getHostingFromShodan(ip string) (string, error) {
    apiKey := "YgCCsdRgTvTDBVUUr4Q5A4vGjjf4CjIG" // Inserire API Key Shodan qui
    client := shodan.NewClient(nil, apiKey)

    // Crea un contesto di base
    ctx := context.Background()

    // Ottieni i servizi dell'IP da Shodan
    host, err := client.GetServicesForHost(ctx, ip, nil)
    if err != nil {
        return "", fmt.Errorf("errore Shodan: %v", err)
    }

    // Verifica i risultati dei nomi host
    if len(host.Hostnames) > 0 {
        return strings.ReplaceAll(fmt.Sprintf("Shodan: %s", host.Hostnames[0]), "\"", ""), nil
    }

    return "Sconosciuto", nil
}

// Funzione per normalizzare il nameserver (rimuovendo prefissi come "ns-cloud-")
func normalizeNameserver(nameserver string) string {
	nameserver = strings.TrimSuffix(nameserver, ".")
	parts := strings.Split(nameserver, ".")
	if len(parts) > 2 {
		nameserver = strings.Join(parts[len(parts)-2:], ".")
	}
	return nameserver
}

func normalizeURL(url string) string {
    // Rimuovi il protocollo
    url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")

    // Dividi per il primo slash
    if idx := strings.Index(url, "/"); idx != -1 {
        url = url[:idx] // Tieni solo la parte prima del primo slash
    }

    // Rimuovi eventuali parametri di query residui
    if idx := strings.Index(url, "?"); idx != -1 {
        url = url[:idx] // Tieni solo la parte prima del punto interrogativo
    }

    return url
}

// Funzione helper per estrarre il dominio dall'URL
func estraiDominio(url string) (string, error) {
    // Rimuovi il protocollo
    urlPulito := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")

    // Dividi per il primo slash per rimuovere il percorso
    parti := strings.Split(urlPulito, "/")
    dominio := parti[0]

    // Gestisci eventuali porti (ad esempio, "www.example.com:8080")
    dominio = strings.Split(dominio, ":")[0]

    // Aggiungi alcuni controlli di validità
    if dominio == "" {
        return "", fmt.Errorf("dominio non valido")
    }

    return dominio, nil
}

// detectCookieBanner controlla la presenza di termini come "Cookie Policy" nella pagina.
func detectCookieBanner(url string) (string, error) {
    client := &http.Client{
        Timeout: 15 * time.Second,
    }

    resp, err := client.Get(url)
    if err != nil {
        return "Non trovato", fmt.Errorf("errore durante il fetch dell'URL %s: %v", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "Non trovato", fmt.Errorf("errore HTTP (%d) per URL: %s", resp.StatusCode, url)
    }

    // Leggi il contenuto HTML
    buf := new(strings.Builder)
    _, err = io.Copy(buf, resp.Body)
    if err != nil {
        return "Non trovato", fmt.Errorf("errore durante la lettura del contenuto HTML: %v", err)
    }
    html := buf.String()

    // Parole chiave per identificare un cookie banner
    keywords := []string{"privacy e cookie policy", "cookie policy", "cookie banner", "cookie settings", "gestione cookie", "accetta i cookie", "impostazioni dei cookie", "dichiarazione privacy"}

    // Controlla la presenza di parole chiave nel contenuto HTML
    for _, keyword := range keywords {
        if strings.Contains(strings.ToLower(html), keyword) {
            return "Sì", nil
        }
    }

    // Se nessuna parola chiave è trovata, restituisci "No"
    return "No", nil
}

// detectProtocol verifica se l'URL usa HTTP o HTTPS, controllando prima l'HTTP e poi l'HTTPS.
func detectProtocol(url string) (string, error) {
    if strings.HasPrefix(url, "https://") {
        return "https", nil
    } else if strings.HasPrefix(url, "http://") {
        return "http", nil
    }

    // Prova HTTPS per primo
    httpsURL := "https://" + strings.TrimPrefix(url, "http://")
    resp, err := http.Get(httpsURL)
    if err == nil && resp.StatusCode == http.StatusOK {
        return "https", nil
    }

    // Se HTTPS fallisce, prova HTTP
    httpURL := "http://" + strings.TrimPrefix(url, "https://")
    resp, err = http.Get(httpURL)
    if err == nil && resp.StatusCode == http.StatusOK {
        return "http", nil
    }

    return "", fmt.Errorf("protocollo non rilevato per URL: %s", url)
}

// detectTechnology analizza la tecnologia usata da un sito web, combinando analisi statica e dinamica.
func detectTechnology(url string, cmsNames map[string][]string) (string, error) {
	// Fase 1: Analisi statica (HTML e headers)
	html, headers := fetchHTML(url)
	detectedCMS := identifyCMS(html, headers, cmsNames)

	// Raffina il rilevamento per casi speciali come ItaliaOnline
	finalCMS := refineCMSDetection(detectedCMS, html, cmsNames)

	// Se il CMS è stato rilevato in modo definitivo, restituiscilo
	if finalCMS != "Altro" {
		return finalCMS, nil
	}

	// Fase 2: Analisi dinamica (browser headless) come fallback
	dynamicTech := dynamicAnalysis(url, cmsNames)

	// Se l'analisi dinamica fornisce un risultato, aggiorna finalCMS
	if dynamicTech != "" && dynamicTech != "Altro" {
		finalCMS = refineCMSDetection(dynamicTech, html, cmsNames)
	}

	// Restituisci il risultato finale (se non rilevato, sarà "Altro")
	return finalCMS, nil
}

// fetchHTML scarica il contenuto HTML di una pagina e le relative intestazioni HTTP.
func fetchHTML(url string) (string, http.Header) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	headers := resp.Header

	// Analizza script esterni per segnali tecnologici
	externalScripts := extractScriptSources(string(body))
	for _, scriptURL := range externalScripts {
		scriptContent := fetchResource(scriptURL)
		if scriptContent != "" {
			body = append(body, []byte(scriptContent)...) // Aggiungi il contenuto degli script all'analisi
		}
	}

	return string(body), headers
}

// Estrai URL dei file script
func extractScriptSources(html string) []string {
	var urls []string
	re := regexp.MustCompile(`<script[^>]+src="([^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		url := match[1]
		if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "./") {
			continue // Ignora percorsi relativi
		}
		urls = append(urls, url)
	}
	return urls
}

// Scarica una risorsa esterna
func fetchResource(url string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}

// identifyCMS identifica la tecnologia utilizzata analizzando l'HTML, le intestazioni e i cookie.
func identifyCMS(html string, headers http.Header, cmsNames map[string][]string) string {
	if html == "" && headers == nil {
		return "Altro"
	}

	// 1. Controllo nell'HTML
	for cms, patterns := range cmsNames {
		for _, pattern := range patterns {
			re := regexp.MustCompile(`(?i)` + pattern) // Case insensitive
			if re.MatchString(html) {
				// Rilevamento specifico per ItaliaOnline
				if cms == "ItaliaOnline" {
					return "ItaliaOnline"
				}
				return cms
			}
		}
	}

	// 2. Controllo negli header HTTP
	if headers != nil {
		for cms, patterns := range cmsNames {
			for _, pattern := range patterns {
				re := regexp.MustCompile(`(?i)` + pattern)
				if re.MatchString(headers.Get("Server")) || re.MatchString(headers.Get("X-Powered-By")) {
					return cms
				}
			}
		}
	}

	// 3. Controllo nei cookie
	cookieHeader := headers.Get("Set-Cookie")
	if cookieHeader != "" {
		for cms, patterns := range cmsNames {
			for _, pattern := range patterns {
				re := regexp.MustCompile(`(?i)` + pattern)
				if re.MatchString(cookieHeader) {
					return cms
				}
			}
		}
	}

	// Nessun CMS rilevato
	return "Altro"
}

func refineCMSDetection(detectedCMS string, html string, cmsNames map[string][]string) string {
	if detectedCMS == "Duda" {
		// Se rilevato "Duda", controlla ulteriormente se ci sono segnali di ItaliaOnline
		iolPatterns, exists := cmsNames["ItaliaOnline"]
		if exists {
			for _, pattern := range iolPatterns {
				re := regexp.MustCompile(`(?i)` + pattern)
				if re.MatchString(html) {
					return "ItaliaOnline"
				}
			}
		}
	}
	return detectedCMS
}

// dynamicAnalysis usa un browser headless per analizzare dinamicamente i contenuti generati.
func dynamicAnalysis(url string, cmsNames map[string][]string) string {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)

    // Configurazione per intercettare richieste
    router := page.HijackRequests()
    router.MustAdd("*", func(ctx *rod.Hijack) {
        requestURL := ctx.Request.URL().String()
        if strings.Contains(requestURL, "wp-json") {
            ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient) // Blocca la richiesta se necessario
        }
    })
    go router.Run()
    defer router.Stop()

    // Caricamento pagina
    html := page.MustHTML()
    headers := fetchHeaders(page)

    // Analisi dinamica
    detectedTech := identifyCMS(html, headers, cmsNames)
    if detectedTech != "Altro" {
        return detectedTech
    }

    return "Altro"
}

// Fetch Headers usando il browser headless
func fetchHeaders(page *rod.Page) http.Header {
	headers := make(http.Header)
	response := page.MustEval(`() => {
        return JSON.stringify({
            userAgent: navigator.userAgent,
            platform: navigator.platform
        });
    }`).String()
	// Converti JSON in Header
	_ = json.Unmarshal([]byte(response), &headers)
	return headers
}

func checkSiteAvailability(url string) (string, error) {
    maxRetries := 5                        // Numero massimo di tentativi
    initialRetryDelay := 5 * time.Second   // Ritardo iniziale tra i tentativi
    maxRetryDelay := 30 * time.Second      // Ritardo massimo tra i tentativi
    timeout := 45 * time.Second            // Timeout per ogni richiesta (aumentato)

    var lastError error
    retryDelay := initialRetryDelay

    for attempt := 1; attempt <= maxRetries; attempt++ {
        client := &http.Client{
            Timeout: timeout,
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= 5 {
                    return http.ErrUseLastResponse // Limita i redirect
                }
                return nil
            },
        }

        resp, err := client.Get(url)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                lastError = fmt.Errorf("tentativo %d: Timeout", attempt)
            } else {
                lastError = fmt.Errorf("tentativo %d: Errore di connessione: %v", attempt, err)
            }
        } else {
            defer resp.Body.Close()

            // Gestione dei codici di stato
            switch resp.StatusCode {
            case http.StatusOK: // 200
                return "Sì", nil
            case http.StatusMovedPermanently, http.StatusFound: // 301, 302
                // Segui i redirect solo se possibile
                redirectURL := resp.Header.Get("Location")
                if redirectURL != "" {
                    url = redirectURL
                    continue
                }
            case http.StatusServiceUnavailable: // 503
                retryAfter := resp.Header.Get("Retry-After")
                if seconds, err := strconv.Atoi(retryAfter); err == nil {
                    time.Sleep(time.Duration(seconds) * time.Second)
                }
                lastError = fmt.Errorf("tentativo %d: Server Temporaneamente Non Disponibile (503)", attempt)
            case http.StatusUnauthorized: // 401
                return "No - Non Autorizzato (401)", nil
            case http.StatusNotFound: // 404
                return "No - Pagina Non Trovata (404)", nil
            default:
                if resp.StatusCode >= 500 {
                    lastError = fmt.Errorf("tentativo %d: Errore Server (%d)", attempt, resp.StatusCode)
                }
            }
        }

        // Incrementa l'attesa con backoff progressivo
        time.Sleep(retryDelay)
        retryDelay = time.Duration(float64(retryDelay) * 1.5) // Aumenta progressivamente
        if retryDelay > maxRetryDelay {
            retryDelay = maxRetryDelay
        }
    }

    // Verifica finale per ridurre i falsi negativi
    finalClient := &http.Client{Timeout: 60 * time.Second} // Timeout più lungo per la verifica finale
    finalResp, finalErr := finalClient.Get(url)
    if finalErr == nil && finalResp.StatusCode == http.StatusOK {
        return "Sì - Disponibile (Verificato)", nil
    }

    // Se tutti i tentativi falliscono
    if lastError != nil {
        return "No - Errore dopo vari tentativi", lastError
    }

    return "Stato Non Determinato", nil
}

func checkSiteMaintenance(html string) string {
	// Parole chiave per la manutenzione o costruzione
	maintenanceKeywords := []string{
		"sito in costruzione", "sito in manutenzione", "site under construction", "maintenance mode",
		"site under maintenance", "site temporarily unavailable", "Modalità di manutenzione", "maintenance-heading",
	}

	// Creazione di una regex per parole/frasi precise
	escapedKeywords := make([]string, len(maintenanceKeywords))
	for i, keyword := range maintenanceKeywords {
		escapedKeywords[i] = regexp.QuoteMeta(keyword) // Escapa i caratteri speciali
	}
	regexPattern := `\b(?:` + strings.Join(escapedKeywords, "|") + `)\b`
	re := regexp.MustCompile(regexPattern)

	// Funzione per estrarre contenuto da tag specifici
	extractText := func(pattern string) string {
		reTag := regexp.MustCompile(pattern)
		matches := reTag.FindAllStringSubmatch(html, -1)
		var extractedContent []string
		for _, match := range matches {
			if len(match) > 1 {
				extractedContent = append(extractedContent, match[1])
			}
		}
		return strings.Join(extractedContent, " ")
	}

	// Estrai contenuti da <title>, <h1> ... <h6>, <p>, <em>, <strong>, <span>, <footer>, <a>
	relevantTags := []string{
		`<title>(.*?)<\/title>`,           // <title>
		`<h1.*?>(.*?)<\/h1>`,              // <h1>
		`<h2.*?>(.*?)<\/h2>`,              // <h2>
		`<h3.*?>(.*?)<\/h3>`,              // <h3>
		`<h4.*?>(.*?)<\/h4>`,              // <h4>
		`<h5.*?>(.*?)<\/h5>`,              // <h5>
		`<h6.*?>(.*?)<\/h6>`,              // <h6>
		`<p.*?>(.*?)<\/p>`,                // <p>
		`<em.*?>(.*?)<\/em>`,              // <em>
		`<strong.*?>(.*?)<\/strong>`,      // <strong>
		`<b.*?>(.*?)<\/b>`,                // <b>
		`<i.*?>(.*?)<\/i>`,                // <i>
		`<span.*?>(.*?)<\/span>`,          // <span>
		`<a.*?>(.*?)<\/a>`,                // <a> (link)
		`<footer.*?>(.*?)<\/footer>`,      // <footer>
		`<div.*?>(.*?)<\/div>`,            // <div>
		`<section.*?>(.*?)<\/section>`,    // <section>
		`<article.*?>(.*?)<\/article>`,    // <article>
		`<mark.*?>(.*?)<\/mark>`,          // <mark>
		`<label.*?>(.*?)<\/label>`,        // <label>
		`<blockquote.*?>(.*?)<\/blockquote>`, // <blockquote>
		`<ins.*?>(.*?)<\/ins>`,            // <ins>
		`<del.*?>(.*?)<\/del>`,            // <del>
	}

	// Cerca le parole chiave nelle sezioni rilevanti del sito
	for _, pattern := range relevantTags {
		content := extractText(pattern)
		if re.MatchString(strings.ToLower(content)) {
			return "Sì"
		}
	}

	// Nessuna corrispondenza trovata
	return "No"
}

func (e *Entry) CsvHeaders() []string {
    return []string{
        "Nome Attività",
        "Categoria",
        "Sito Web",
        "Telefono",
        "Indirizzo",
        "Comune",
        "Provincia",
        "Email",
        "Protocollo",
        "Tecnologia",
        "Cookie Banner",
        "Hosting Provider",
        "Performance Mobile",
        "Performance Desktop",
        "Punteggio SEO",
        "Disponibilità Sito",  // Nuovo campo
        "Stato Manutenzione",  // Nuovo campo
    }
}

// Funzione per creare la riga CSV con tutti i dettagli, incluso il provider di hosting
func (e *Entry) CsvRow(excludedWebsites map[string]struct{}, providerFile string) []string {
    // Controlla che almeno uno dei campi principali non sia vuoto
    if e.Title == "" && e.WebSite == "" && e.Phone == "" && e.Email == "" && e.Street == "" && e.City == "" &&
        e.Province == "" && e.Protocol == "" && e.Technology == "" && e.CookieBanner == "" && e.HostingProvider == "" &&
        e.MobilePerformance == "" && e.DesktopPerformance == "" && e.SeoScore == "" && e.SiteAvailability == "" &&
        e.SiteMaintenance == "" {
        return nil // Se tutti i campi sono vuoti, non genera la riga
    }

    if e.Title == "" || e.Phone == "" {
        return nil
    }

    // Verifica se il sito web è escluso o è un social media
    dominio := e.WebSite
    if dominio == "" || isExcludedWebsite(dominio, excludedWebsites) || isSocialMediaDomain(dominio) {
        dominio = "" // Se il sito è escluso o appartiene ai social media, azzera il campo
    }

    // Normalizza il dominio rimuovendo i parametri di query
    dominio = normalizeURL(dominio)

    // Aggiungi il provider di hosting solo se il dominio non è escluso
    hostingProvider := e.HostingProvider
    if dominio != "" {
        var err error
        hostingProvider, err = getHostingProviderWithFile(dominio, providerFile) // Chiamata per ottenere il provider
        if err != nil {
            hostingProvider = "Sconosciuto"
        }
    }

    // Genera la riga CSV con i campi ripuliti dalle virgolette doppie
    row := []string{
        e.Title,
        e.Category,
        dominio,
        e.Phone,
        e.Street,
        e.City,
        e.Province,
        e.Email,
        e.Protocol,
        e.Technology,
        e.CookieBanner,
        hostingProvider,
        e.MobilePerformance,
        e.DesktopPerformance,
        e.SeoScore,
        e.SiteAvailability,
        e.SiteMaintenance,
    }

    // Rimuovi le virgolette doppie da ogni campo
    for i, field := range row {
        row[i] = strings.ReplaceAll(field, "\"", "")
    }

    return row
}

// Funzione per verificare se un dominio è un social media
func isSocialMediaDomain(domain string) bool {
    socialMediaPatterns := []string{
        "www.facebook.com",
        "facebook.com",
        "www.instagram.com",
        "instagram.com",
        "www.linkedin.com",
        "linkedin.com",
        "whatsapp.com",
        "wa.me",
        "linkedin.",
        "instagram.",
        "facebook.",
        "whatsapp.",
    }

    // Controlla se il dominio contiene uno dei pattern social media
    for _, pattern := range socialMediaPatterns {
        if strings.Contains(domain, pattern) {
            return true
        }
    }

    return false
}

func EntryFromJSON(raw []byte, cmsFile, excludeFile, providerFile string) (Entry, error) {
    var entry Entry
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("Recovered from panic: %v\nStack: %s\n", r, debug.Stack())
        }
    }()

    // Carica i file CMS
    cmsNames, err := loadCmsNames(cmsFile)
    if err != nil {
        return entry, fmt.Errorf("errore nel caricare i CMS names: %v", err)
    }

    // Carica i siti esclusi
    excludedWebsites, err := loadExcludedWebsites(excludeFile)
    if err != nil {
        return entry, fmt.Errorf("errore nel caricare i siti esclusi: %v", err)
    }

    // Decodifica il JSON
    var jd []any
    if err := json.Unmarshal(raw, &jd); err != nil {
        return entry, err
    }

    // Verifica che la struttura JSON sia corretta
    if len(jd) < 7 {
        return entry, fmt.Errorf("json non valido")
    }

    // Ottieni l'array dai dati
    darray, ok := jd[6].([]any)
    if !ok {
        return entry, fmt.Errorf("struttura json non valida")
    }

    // Estrai i dati
    entry.Title, err = getNthElementAndCast[string](darray, 11)
    if err != nil {
        return entry, err
    }

    entry.Category, err = getNthElementAndCast[string](darray, 13, 0)
    if err != nil {
        return entry, err
    }

    // Se il sito web non esiste, lascialo vuoto e gestisci gli altri campi di conseguenza
    entry.WebSite, err = getNthElementAndCast[string](darray, 7, 0)
    if err != nil || entry.WebSite == "" {
        entry.WebSite = "" // Lascia vuoto se il sito non esiste
    }

    // Verifica se il sito web è escluso
    if isExcludedWebsite(entry.WebSite, excludedWebsites) {
        entry.WebSite = "" // Se escluso, metti "N/A"
    }

    entry.Phone, err = getNthElementAndCast[string](darray, 178, 0, 0)
    if err != nil {
        return entry, err
    }

    // Estrai l'indirizzo
    entry.Street, err = getNthElementAndCast[string](darray, 183, 1, 1)
    if err != nil {
        return entry, err
    }

    entry.City, err = getNthElementAndCast[string](darray, 183, 1, 3)
    if err != nil {
        return entry, err
    }

    // Rimuovi "Province of" da State se presente
    entry.Province = strings.Replace(entry.Province, "Province of ", "", 1)

    // Estrai i dati SEO e di hosting solo se il sito web è valido
    if entry.WebSite != "" && entry.WebSite != "N/A" {
        if protocol, err := detectProtocol(entry.WebSite); err == nil {
            entry.Protocol = protocol
        }
        if technology, err := detectTechnology(entry.WebSite, cmsNames); err == nil && technology != "" {
            entry.Technology = technology
        } else {
            entry.Technology = "Altro"
        }    
        if cookieBanner, err := detectCookieBanner(entry.WebSite); err == nil {
            entry.CookieBanner = cookieBanner
        } else {
            fmt.Printf("Errore rilevazione cookie banner per %s: %v\n", entry.WebSite, err)
            entry.CookieBanner = "Non trovato" // Assegna valore predefinito se non trovato
        }
        if hostingProvider, err := getHostingProviderWithFile(entry.WebSite, providerFile); err == nil {
            entry.HostingProvider = hostingProvider
        } else {
            entry.HostingProvider = "Sconosciuto"
        }

        if mobilePerf, desktopPerf, seoScore, err := getPageSpeedScores(entry.WebSite); err == nil {
            entry.MobilePerformance = strconv.Itoa(mobilePerf)
            entry.DesktopPerformance = strconv.Itoa(desktopPerf)
            entry.SeoScore = strconv.FormatFloat(seoScore, 'f', 2, 64)
        }

        // Check Disponibilità e Manutenzione
        if availability, err := checkSiteAvailability(entry.WebSite); err == nil {
            entry.SiteAvailability = availability
        }

        if html, _ := fetchHTML(entry.WebSite); html != "" {
            entry.SiteMaintenance = checkSiteMaintenance(html)
        }
    }

    return entry, nil
}

// isExcludedWebsite verifica se il sito web deve essere escluso
func isExcludedWebsite(url string, excludedWebsites map[string]struct{}) bool {
    // Helper per rimuovere il prefisso "www."
    removeWWW := func(domain string) string {
        return strings.TrimPrefix(domain, "www.")
    }

    // Helper per verificare i suffissi (wildcard parziali)
    matchesExcludedPattern := func(domain string, excludedWebsites map[string]struct{}) bool {
        for excluded := range excludedWebsites {
            if strings.HasPrefix(domain, excluded) || strings.HasSuffix(domain, excluded) {
                return true
            }
        }
        return false
    }    

    // Estrai il dominio principale dall'URL
    domain, err := estraiDominio(url)
    if err != nil {
        return false // Se non riesce a estrarre il dominio, considera il sito non escluso
    }

    // Rimuovi il prefisso "www." dal dominio
    domainWithoutWWW := removeWWW(domain)

    // Controlla se il dominio o il dominio senza "www." è esattamente nella lista esclusa
    if _, found := excludedWebsites[domain]; found {
        fmt.Printf("Sito escluso (esatto nella lista esclusi): %s\n", url)
        return true
    }
    if _, found := excludedWebsites[domainWithoutWWW]; found {
        fmt.Printf("Sito escluso (esatto senza www): %s\n", url)
        return true
    }

    // Controlla se il dominio o il dominio senza "www." corrisponde a un pattern escluso (wildcard parziale)
    if matchesExcludedPattern(domain, excludedWebsites) || matchesExcludedPattern(domainWithoutWWW, excludedWebsites) {
        fmt.Printf("Sito escluso (wildcard parziale): %s\n", url)
        return true
    }

    // Verifica contro domini social o specifici
    if isSocialOrSpecificDomain(domain) || isSocialOrSpecificDomain(domainWithoutWWW) {
        fmt.Printf("Sito escluso (social o specifico): %s\n", url)
        return true
    }

    // Verifica contro domini con prefissi specifici
    if hasForbiddenPrefix(domain) || hasForbiddenPrefix(domainWithoutWWW) {
        fmt.Printf("Sito escluso (prefisso specifico): %s\n", url)
        return true
    }

    // Verifica contro estensioni particolari
    if hasForbiddenExtension(domain) || hasForbiddenExtension(domainWithoutWWW) {
        fmt.Printf("Sito escluso (estensione specifica): %s\n", url)
        return true
    }

    return false // Il sito non è escluso
}

func isSocialOrSpecificDomain(domain string) bool {
    // Lista dei domini o parole da escludere
    keywords := []string{
        "facebook.", "instagram.", "linkedin.", "youtube.", "tiktok.",
        "comune.", "e-coop.it", ".iqos.", ".tecnocasa.",
        "bookizon.it", "widget.treatwell.it", "treatwell.it",
    }

    for _, keyword := range keywords {
        if strings.Contains(domain, keyword) {
            return true
        }
    }

    return false
}

func hasForbiddenPrefix(domain string) bool {
    prefixes := []string{"lecce", "centrocommerciale"}

    for _, prefix := range prefixes {
        if strings.HasPrefix(strings.ToLower(domain), prefix) {
            return true
        }
    }

    return false
}

func hasForbiddenExtension(domain string) bool {
    extensions := []string{".edu.it", ".fr"}

    for _, ext := range extensions {
        if strings.HasSuffix(strings.ToLower(domain), ext) {
            return true
        }
    }
    return false
}

func validateEmail(email string) string {
	if len(email) > 100 {
		return ""
	}
	regex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !regex.MatchString(email) {
		return ""
	}
	return email
}

func getNthElementAndCast[T any](arr []any, indexes ...int) (T, error) {
	var defaultVal T
	for len(indexes) > 1 {
		idx := indexes[0]
		indexes = indexes[1:]
		if idx >= len(arr) {
			return defaultVal, fmt.Errorf("indice fuori dal range: %d", idx)
		}
		next, ok := arr[idx].([]any)
		if !ok {
			return defaultVal, fmt.Errorf("tipo non corrispondente per l'indice: %d", idx)
		}
		arr = next
	}
	if len(indexes) == 0 || indexes[0] >= len(arr) {
		return defaultVal, fmt.Errorf("indice fuori dal range: %d", indexes[0])
	}
	value, ok := arr[indexes[0]].(T)
	if !ok {
		return defaultVal, fmt.Errorf("impossibile convertire il valore in tipo %T", defaultVal)
	}
	return value, nil
}