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
	"bufio"
	"os"

	
	"github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/proto"
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
    excluded := make(map[string]struct{})
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("errore nell'aprire il file %s: %v", filename, err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        domain := scanner.Text()
        excluded[domain] = struct{}{}
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("errore durante la lettura del file %s: %v", filename, err)
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

// Funzione per ottenere il provider di hosting a partire dal dominio
func getHostingProvider(domain string) (string, error) {
    // Estrai il dominio dall'URL
    parsedDomain, err := estraiDominio(domain)
    if err != nil {
        return "Sconosciuto", err
    }

    // Prova a trovare i nameservers per il dominio
    nameservers, err := net.LookupNS(parsedDomain)
    if err != nil {
        return "Sconosciuto", err
    }

    // Se ci sono nameservers, prova a identificare l'hosting
    for _, ns := range nameservers {
        hostingProvider := identificaHostingDaNameserver(ns.Host)
        if hostingProvider != "Sconosciuto" {
            return hostingProvider, nil
        }
    }

    return "Sconosciuto", nil
}

// Funzione per normalizzare il nameserver (rimuovendo prefissi come "ns-cloud-")
func normalizeNameserver(nameserver string) string {
    prefixes := []string{
        "ns-cloud-", "dns1.", "dns2.", "dns3.", "dns4.", "ns1.", "ns2.", "ns3.", "ns4.",
    }

    for _, prefix := range prefixes {
        if strings.HasPrefix(nameserver, prefix) {
            nameserver = strings.TrimPrefix(nameserver, prefix)
            break
        }
    }

    // Rimuovi solo suffissi generici come `.com.` o `.net.` se presenti
    if strings.HasSuffix(nameserver, ".") {
        nameserver = strings.TrimSuffix(nameserver, ".")
    }

    return nameserver
}


// Funzione helper per estrarre il dominio dall'URL
func estraiDominio(url string) (string, error) {
    // Rimuovi il protocollo
    urlPulito := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
    
    // Dividi per il primo slash per rimuovere il percorso
    parti := strings.Split(urlPulito, "/")
    dominio := parti[0]

    // Rimuovi www. se presente
    dominio = strings.TrimPrefix(dominio, "www.")

    // Aggiungi alcuni controlli di validità
    if dominio == "" {
        return "", fmt.Errorf("dominio non valido")
    }

    return dominio, nil
}

// Funzione di fallback per identificare l'hosting
func identificaHostingDaNameserver(nameserver string) string {
    providerMapping := map[string]string{
    "iweblab.it": "iWebLab-Hosting",
    "widhost.net": "WIDHost",
    "register.it": "Register.it-Hosting",
    "seeweb.it": "Seeweb",
    "sideralia.it": "Sideralia-Hosting",
    "technorail.com": "Aruba-Hosting-(Technorail)",
    "keliweb.eu": "Keliweb",
    "netsons.net": "Netsons", 
    "netsons.com": "Netsons",
    "ormag.info": "Ormag-Hosting",
    "abdns.biz": "ABDns-Hosting",
    "secureserver.net": "GoDaddy-Hosting",
    "italianserverlocation.com": "Italian-Server-Location",
    "wixdns.net": "Wix",
    "googledomains.com": "Google-Cloud",
    "siteground.net": "SiteGround",
    "arubadns.cz": "Aruba-Hosting",
    "dns.technorail.com": "Aruba-DNS",
    "dns2.technorail.com": "Aruba-DNS",
    "dns3.arubadns.net": "Aruba-DNS",
    "dns4.arubadns.cz": "Aruba-DNS",
    "cloudflare.com": "Cloudflare",
    "websitehostingserver.com": "Website-Hosting-Server",
    "jimdo.com": "Jimdo-Hosting",
    "litespeedtech.com": "LiteSpeed-Technologies",
    "digitalocean.com": "DigitalOcean",
    "awsdns.com": "Amazon-Web-Services-(AWS)",
    "azure.com": "Microsoft-Azure",
    "linode.com": "Linode",
    "vultr.com": "Vultr",
    "cdn77.com": "CDN77",
    "fastly.com": "Fastly-CDN",
    "stackpath.com": "StackPath-CDN",
    "keycdn.com": "KeyCDN",
    "cloudfront.net": "Amazon-CloudFront",
    "cloudflare.net": "Cloudflare-CDN",
    "akamai.com": "Akamai-CDN",
    "rackspace.com": "Rackspace",
    "gcp.com": "Google-Cloud-Platform-(GCP)",
    "gcloud.com": "Google-Cloud-Platform-(GCP)",
    "cloud.google.com": "Google-Cloud",
    "ovh.net": "OVH-Hosting",
    "hetzner.com": "Hetzner-Online",
    "bluehost.com": "BlueHost",
    "dreamhost.com": "DreamHost",
    "hostgator.com": "HostGator",
    "1and1.com": "1&1-IONOS",
    "namecheap.com": "Namecheap-Hosting",
    "hostwinds.com": "Hostwinds",
    "contabo.com": "Contabo-Hosting",
    "digitaloceanspaces.com": "DigitalOcean-Spaces",
    "rackcdn.com": "Rackspace-CDN",
    "bitnami.com": "Bitnami-Hosting",
    "opensrs.net": "OpenSRS-DNS",
    "googleservletengine.com": "Google-Servlet-Engine",
    "wordpress.com": "WordPress.com-DNS",
    "dnssec.org": "DNSSEC",
    "amazonses.com": "Amazon-Simple-Email-Service-(SES)",
    "netsolhost.com": "Network-Solutions-Hosting",
    "inmotionhosting.com": "InMotion-Hosting",
    "liquidweb.com": "Liquid-Web-Hosting",
    "kinsta.com": "Kinsta-WordPress-Hosting",
    "pagely.com": "Pagely-WordPress-Hosting",
    "cloudways.com": "Cloudways-Managed-Hosting",
    "netlify.com": "Netlify-Web-Hosting",
    "vercel.com": "Vercel-Cloud-Platform",
    "heroku.com": "Heroku-Cloud-Platform",
    "ibm.com": "IBM-Cloud",
    "oracle.com": "Oracle-Cloud",
    "softlayer.com": "IBM-SoftLayer",
    "godaddy.net": "GoDaddy-DNS",
    "cloudns.net": "CloudNS-DNS",
    "dynectdns.com": "Dyn-DNS-(Oracle)",
    "route53.com": "Amazon-Route-53-DNS",
    "azureedge.net": "Microsoft-Azure-CDN",
    "godaddy.gom": "Go-Daddy",
    "azure-mobile.net": "Azure-Mobile-Services",
    "azure-api.net": "Azure-API-Management",
    "windowsazure.com": "Microsoft-Azure",
    "zerigo.net": "Zerigo-DNS",
    "netdna.com": "NetDNA-CDN",
    "edgekey.net": "Akamai-EdgeKey",
    "leaseweb.net": "LeaseWeb-Hosting",
    "googlehosted.com": "Google-Hosted-Services",
    "unifiedlayer.com": "Unified-Layer-Hosting",
    "webhostingpad.com": "WebHostingPad",
    "fatcow.com": "FatCow-Hosting",
    "cyberdyne.cloud": "Cyberdyne-Cloud-Services",
    "ionos.com": "IONOS-Hosting",
    "hostpapa.com": "HostPapa",
    "a2hosting.com": "A2-Hosting",
    "greengeeks.com": "GreenGeeks-Hosting",
    "wpengine.com": "WP-Engine",
    "pressable.com": "Pressable-WordPress-Hosting",
    "mediatemple.net": "Media-Temple",
    "godaddy.cloud": "GoDaddy-Cloud",
    "hostinger.com": "Hostinger",
    "interserver.net": "InterServer",
    "hostiso.com": "Host-ISO",
    "equinix.com": "Equinix-Cloud",
    "serverplan.com": "ServerPlan",
    "aruba.cloud": "Aruba-Cloud",
    "cdn.net": "Generic-CDN-Services",
    "cloudhost.io": "Cloud-Host",
    "servage.net": "Servage-Hosting",
    "strato.de": "STRATO-Hosting",
    "ionos.cloud": "IONOS-Cloud",
    "datacenter.it": "Italian-Data-Center",
    "clouditalia.com": "Cloud-Italia",
    "webhost.it": "Web-Host-Italia",
    "registerit.cloud": "Register.it-Cloud",
    "servercloud.it": "Server-Cloud-Italia",
    "cloudflare.workers.dev": "Cloudflare-Workers",
    "render.com": "Render-Cloud-Platform",
    "fly.io": "Fly.io-Deployment-Platform",
    "railway.app": "Railway-App-Hosting",
    "cyclic.sh": "Cyclic-Hosting",
    "northflank.com": "Northflank-Cloud-Platform",
    "supabase.com": "Supabase-Hosting",
    "platform.sh": "Platform.sh-Cloud-Hosting",
    "ns1.register.it": "Register.it-Hosting",
    "ns2.register.it": "Register.it-Hosting",
    "ns1.siteground.net": "SiteGround",
    "ns2.siteground.net": "SiteGround",
    "ns1.th.seeweb.it": "Seeweb",
    "ns2.th.seeweb.it": "Seeweb",
	"openprovider.com": "OpenProvider-DNS",
	"server.it": "Server-IT", 
	"supporthost.com": "SupportHost", 
	"altervista.org": "Altervista", 
	"host.it": "Host-Italia", 
	"web.com": "Web.com", 
	"vhosting.it": "VHosting", 
	"shellrent.it": "ShellRent", 
	"artera.com": "Artera", 
	"hosting4agency.com": "Hosting4Agency", 
	"tophost.it": "TopHost", 
	"flamenetworks.com": "FlameNetworks", 
	"webhosting.it": "WebHosting", 
	"hostingsolutions.it": "HostingSolutions", 
	"utixo.com": "Utixo", 
    "hostingperte.it": "Hosting-Per-Te",
    "misterdomain.eu": "Mister Domain",
    "domaincontrol.com": "GoDaddy-DNS",
    "cmshigh.com": "ServerPlan",
    "sphostserver.com": "ServerPlan",
    "dnsparking.com": "Hostinger",
    "dondominio.com": "Don-Dominio",
    "webempresa.eu": "Don-Dominio",
    "mydnsdomains.com": "Tucows Domains",
    "tol.it": "Aruba",
    "vhosting-it.com": "VHosting",
    "ui-dns.com": "1&1-IONOS",
    "ui-dns.org": "1&1-IONOS",
    "ui-dns.de": "1&1-IONOS",
    "ui-dns.biz": "1&1-IONOS",
    "aruba.it": "Aruba-Hosting",
    "paginesi.it": "Pagine Sì",
    "serverdomus.com": "SeeWeb",
    "ns-551.awsdns-04.net": "Amazon Web Services",
    "ns-1162.awsdns-17.org": "Amazon Web Services",
    "ns-2001.awsdns-58.co.uk": "Amazon Web Services",
    "ns-284.awsdns-35.com": "Amazon Web Services",
    "awsdns.net": "Amazon Web Services",
    "awsdns.org": "Amazon Web Services",
    "awsdns.co.uk": "Amazon Web Services",
    "incubatec.net": "Incubatec Hosting",
    "qubus.it": "Qubus Hosting",
    "edis.global": "Edis Global",
    "it-service.bz.it": "InterNetX",
    "dnsitalia.net": "Hetzner",
    "hostcsi.com": "HostCSI",
    "flamedns.host": "Seeweb",
    "serverkeliweb.it": "Keliweb",
    "dnshigh.com": "Serverplan",
    "host-anycast.com": "Netsons",
    "namecheaphosting.com": "Namecheap",
    "infomaniak.com": "Infomaniak",
    "dominiok.it": "Hostinger",
    "altervista.com": "Altervista",
    "limecloud.it": "LimeCloud",
    "server24.eu": "Server24",
    "omnibus.net": "OVH",
    "pianetaitalia.com": "Pianeta Italia",
    "one.com": "One.com-Hosting",
    "fol.it": "Fol-it Hosting",
    "ovhcloud.com": "OVH",
    "ovh.it": "OVH",
    "contabo.net": "Contabo",
    "mvnet.com": "MVNet",
    "mvnet.it": "MVNet",
    "mvnet-dns.eu": "MVNet",
    "interferenza.it": "Interferenza Hosting",
    "interferenza.net": "Interferenza Hosting",
    "easygreenhosting.it": "Easy Green Hosting",
    "kreativmedia.ch": "KreativMedia Hosting",
    "ricpic.com": "Provider con OVH",
    "pop.it": "Provider con Aruba",

    }

    for key, provider := range providerMapping {
        if strings.Contains(nameserver, key) {
            return provider
        }
    }

    return "Sconosciuto"
}

// detectCookieBanner controlla la presenza della parola "Cookie Policy" nella pagina.
func detectCookieBanner(url string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "No", fmt.Errorf("errore durante il fetch dell'URL %s: %v", url, err)
	}
	defer resp.Body.Close()

	// Leggi il contenuto HTML
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "No", fmt.Errorf("errore durante la lettura del contenuto HTML: %v", err)
	}
	html := buf.String()

	// Controlla la presenza di "Cookie Policy"
	if strings.Contains(strings.ToLower(html), "cookie policy") {
		return "Sì", nil
	}

	return "No", nil
}

// detectProtocol verifica se l'URL usa HTTP o HTTPS, controllando prima l'HTTP e poi l'HTTPS.
func detectProtocol(url string) (string, error) {
    // Se l'URL inizia con https://, usa direttamente https
    if strings.HasPrefix(url, "https://") {
        return "https", nil
    } else if strings.HasPrefix(url, "http://") {
        // Se l'URL inizia con http://, controlla se https:// è disponibile
        httpsURL := strings.Replace(url, "http://", "https://", 1)
        resp, err := http.Get(httpsURL)
        if err != nil || resp.StatusCode != http.StatusOK {
            return "http", nil // Se HTTPS non è disponibile, usa HTTP
        }
        return "https", nil // Se HTTPS è disponibile, usa HTTPS
    }

    // Se l'URL non contiene nessuno dei due protocolli, prova prima con HTTPS, poi con HTTP
    httpsURL := "https://" + url
    resp, err := http.Get(httpsURL)
    if err != nil || resp.StatusCode != http.StatusOK {
        // Se HTTPS non è disponibile, prova con HTTP
        httpURL := "http://" + url
        resp, err = http.Get(httpURL)
        if err != nil || resp.StatusCode != http.StatusOK {
            return "", fmt.Errorf("protocollo non rilevato per URL: %s", url)
        }
        return "http", nil
    }
    return "https", nil // Se HTTPS è disponibile, usa HTTPS
}

// detectTechnology analizza la tecnologia usata da un sito web, combinando analisi statica e dinamica.
func detectTechnology(url string, cmsNames map[string][]string) (string, error) {
	// Analisi statica (HTML e intestazioni)
	html, headers := fetchHTML(url)
	tech := identifyCMS(html, headers, cmsNames) // Passa la mappa cmsNames
	if tech != "" {
		return tech, nil
	}

	// Analisi dinamica (browser headless)
	dynamicTech := dynamicAnalysis(url, cmsNames)
	if dynamicTech != "" {
		return dynamicTech, nil
	}

	return "Altro", nil
}

// fetchHTML scarica il contenuto HTML di una pagina e le relative intestazioni HTTP.
func fetchHTML(url string) (string, http.Header) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	headers := resp.Header

	// Scarica risorse esterne menzionate
	externalScripts := extractScriptSources(string(body))
	for _, scriptURL := range externalScripts {
		go fetchResource(scriptURL) // Scarica asincronicamente
	}

	return string(body), headers
}

// Estrai URL dei file script
func extractScriptSources(html string) []string {
	var urls []string
	re := regexp.MustCompile(`<script[^>]+src="([^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		urls = append(urls, match[1])
	}
	return urls
}

// Scarica una risorsa esterna
func fetchResource(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(resp.Body)
	return string(content)
}

// identifyCMS identifica la tecnologia utilizzata analizzando l'HTML, le intestazioni e i cookie.
func identifyCMS(html string, headers http.Header, cmsNames map[string][]string) string {
	if html == "" && headers == nil {
		return "Errore: HTML e intestazioni non disponibili"
	}

	// Verifica nell'HTML
	for cms, patterns := range cmsNames {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(html) {
				return cms
			}
		}
	}

	// Verifica negli header HTTP
	if headers != nil {
		headerPatterns := map[string]string{
			"WordPress": "WordPress",
			"Django":    "Django",
			"ASP.NET":   "ASP.NET",
			"Shopify":   "Shopify",
			"Magento":   "Magento",
			"Joomla":    "Joomla",
		}
		for cms, pattern := range headerPatterns {
			if strings.Contains(headers.Get("Server"), pattern) || strings.Contains(headers.Get("X-Powered-By"), pattern) {
				return cms
			}
		}
	}

	// Verifica nei cookie
	cookieHeader := headers.Get("Set-Cookie")
	if cookieHeader != "" {
		if strings.Contains(cookieHeader, "wordpress") {
			return "WordPress"
		}
	}

	// Se nessun CMS è stato trovato
	return "Altro"
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
	response := page.MustEval(`() => { return JSON.stringify([...navigator]) }`).String()
	// Converti JSON in Header
	_ = json.Unmarshal([]byte(response), &headers)
	return headers
}

func checkSiteAvailability(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "No", fmt.Errorf("errore: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "Yes", nil
	}

	switch resp.StatusCode {
	case 404:
		return "No - 404 Not Found", nil
	case 500:
		return "No - 500 Internal Server Error", nil
	default:
		return fmt.Sprintf("No - Errore %d", resp.StatusCode), nil
	}
}

func checkSiteMaintenance(html string) string {
	// Parole chiave comuni per indicare manutenzione o costruzione
	maintenanceKeywords := []string{
		"in costruzione", "manutenzione", "under construction", "maintenance mode",
		"work in progress", "coming soon", "site under maintenance", "temporarily unavailable",
	}

	// Cerca parole chiave nel contenuto HTML (corpo della pagina)
	for _, keyword := range maintenanceKeywords {
		if strings.Contains(strings.ToLower(html), keyword) {
			return "Yes"
		}
	}

	// Cerca nel tag <title>
	re := regexp.MustCompile(`<title>(.*?)<\/title>`)
	match := re.FindStringSubmatch(strings.ToLower(html))
	if len(match) > 1 {
		title := match[1]
		for _, keyword := range maintenanceKeywords {
			if strings.Contains(title, keyword) {
				return "Yes"
			}
		}
	}

	// Verifica nel tag <meta> per metadati relativi alla manutenzione
	metaTags := []string{
		`<meta name="maintenance" content="true">`,
		`<meta name="status" content="maintenance">`,
	}

	for _, meta := range metaTags {
		if strings.Contains(strings.ToLower(html), meta) {
			return "Yes"
		}
	}

	// Cerca anche nel tag <body> per altre parole chiave
	reBody := regexp.MustCompile(`<body.*?>(.*?)<\/body>`)
	bodyMatch := reBody.FindStringSubmatch(html)
	if len(bodyMatch) > 1 {
		bodyContent := bodyMatch[1]
		for _, keyword := range maintenanceKeywords {
			if strings.Contains(strings.ToLower(bodyContent), keyword) {
				return "Yes"
			}
		}
	}

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
func (e *Entry) CsvRow(excludedWebsites map[string]struct{}) []string {
    if e.Title == "" || e.Phone == "" {
        return nil
    }

    // Verifica se il sito web è escluso
    dominio := e.WebSite
    if dominio == "" || isExcludedWebsite(dominio, excludedWebsites) {
        dominio = ""  // Se il sito è escluso, mettiamo "N/A" nel campo del sito web
    }

    // Aggiungi il provider di hosting
    hostingProvider := e.HostingProvider
    if dominio != "" {
        hostingProvider, _ = getHostingProvider(dominio)  // Chiamata per ottenere il provider
    }

    return []string{
        e.Title,
        e.Category,
        dominio,  // Se il sito è escluso, "N/A" viene inserito
        e.Phone,
        e.Street,
        e.City,
        e.Province,
        e.Email,
        e.Protocol,
        e.Technology,
        e.CookieBanner,
        hostingProvider,  // Aggiungi il nameserver
        e.MobilePerformance,
        e.DesktopPerformance,
        e.SeoScore,       // Aggiungi il punteggio SEO
        e.SiteAvailability, // Disponibilità del sito
        e.SiteMaintenance,  // Stato di manutenzione
    }
}

// Funzione aggiornata per creare un Entry
func EntryFromJSON(raw []byte, cmsFile, excludeFile string) (Entry, error) {
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
        entry.WebSite = ""  // Lascia vuoto se il sito non esiste
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

    // Estrai i dati SEO solo se il sito web è valido
    if entry.WebSite != "" && entry.WebSite != "N/A" {
        if protocol, err := detectProtocol(entry.WebSite); err == nil {
            entry.Protocol = protocol
        }
        if technology, err := detectTechnology(entry.WebSite, cmsNames); err == nil {
            entry.Technology = technology
        }
        if cookieBanner, err := detectCookieBanner(entry.WebSite); err == nil {
            entry.CookieBanner = cookieBanner
        }

        if hostingProvider, err := getHostingProvider(entry.WebSite); err == nil {
            entry.HostingProvider = hostingProvider
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
    // Estrai il dominio principale dall'URL
    domain, err := estraiDominio(url)
    if err != nil {
        return false // Se non riesce a estrarre il dominio, considera il sito non escluso
    }

    // Lista dei domini dei social media da escludere
    socialMediaDomains := []string{
        "matrimonio.com",
        "bricoman.it",
        "tecnomat.it",
        "apple.com",
        "arcaplanet.it",
        "canon.it",
        "globo.it",
        "burgerking.it",
        "maisonsdumonde.com",
        "kfc.it",
        "hm.com",
        "happycasa.com",
        "sonnybono.com",
        "levis.com",
        "meltingpot.it",
        "vikingop.it",
        "dhl.com",
        "gls-italy.com",
        "ups.com",
        "fedex.com",
        "miodottore.com",
        "instagram.com",
        "facebook.com",
        "www.facebook.com",
        "booking.com",
        "airbnb.com",
        "linkedin.com",
        "twitter.com",
        "youtube.com",
        "pinterest.com",
        "tripadvisor.com",
        "tiktok.com",
        "wix.com",
        "trivago.com",
        "squarespace.com",
        "godaddy.com",
        "weebly.com",
        "tumblr.com",
        ".edu.it",
        "calendar.app.google",
        "linktr.ee",
        "dottori.it",
        "sanita.",
        "fb.me",
        "poste.it",
        "bianalisi.it",
        "sisal.it",
        "eurobet.it",
        "betfair.it",
        "roulette.com",
        "gioco.it",
        "scommesse.it",
        "casino.com",
        "supermercatidok.it",
        "mondadoristore.it",
        "amazon.it",
        "prenatal.com",
        "aeo.it",
        "toyscenter.it",
        "coop.it",
        "conad.it",
        "visionottica.it",
        "amplifon.it",
        "grandivision.it",
        "wa.me",
        "ebay.it",
        "aliexpress.com",
        "zalando.it",
        "asos.com",
        "kayak.com",
        "kayak.it",
        ".gov.it",
        "sephora.it",
        "douglas.it",
        "yves-rocher",
        "tigota.it",
        "welinkbuilders.it",
        "lidl",
        "eurospin.it",
        "despar.it",
        "mdspa.it",
        "aw-lab.com",
        "footlocker.it",
        "whatsapp.com",
        "telegram.me",
        "viber.com",
        "shein.com",
        "etsy.com",
        "wish.com",
        "ikea.com",
        "leroymerlin.it",
        "mediaworld.it",
        "unieuro.it",
        "trony.it",
        "decathlon.it",
        "decathlon.com",
        "euronics.it",
        "pullandbear.com",
        "zalandoprive.it",
        "stradivarius.com",
        "mcdonalds.it",
        "alcott.eu",
        "mongolfieralecce.it",
        "auchan.fr",
        "intimissimi.com",
        "bershka.com",
        "zara.com",
        "snai.it",
        "bwin.it",
        "starcasino.it",
        "leovegas.it",
        "betway.it",
        "netflix.com",
        "disneyplus.com",
        "primevideo.com",
        "spotify.com",
        "expedia.com",
        "lastminute.com",
        "skyscanner.it",
        "italo.it",
        "ryanair.com",
        "easyjet.com",
        "inps.it",
        "agenziaentrate.gov.it",
        "anagrafe.it",
        "ovs.it",
        "coursera.org",
        "chicco.com",
        "casanovadesign.it",
        "casaamica.it",
        "buffetti.it",
        "poste.it",
        "lg.com",
        "samsung.com",
        "bartolini.it",
        "fiat.it",
        "lancia.it",
        "spaziocasa.it",
        "mango.com",
        "volkswagen.it",
        "bmw.it",
        "audi.it",
        "mercedes-benz.it",
        "peugeot.it",
        "renault.it",
        "toyota.it",
        "hyundai.it",
        "nissan.it",
        "kia.com",
        "seat.it",
        "skoda-auto.it",
        "jeep.it",
        "tesla.com",
        "maserati.com",
        "lamborghini.com",
        "ferrari.com",
        "dsautomobiles.it",
        "uniqlo.com",
        "gap.com",
        "banana-republic.com",
        "topshop.com",
        "calvinklein.com",
        "tommy.com",
        "diesel.com",
        "northface.com",
        "timberland.it",
        "rolex.com",
        "napapijri.com",
        "lacoste.com",
        "poliziadistato.it",
        "carabinieri.it",
        "visitlecce.eu",
        "aldi.it",
        "penny.it",
        "tigerstores.it",
        "cooponline.it",
        "esselunga.it",
        "asus.com",
        "lenovo.com",
        "acer.com",
        "dell.com",
        "hp.com",
        "nike.com",
        "adidas.com",
        "underarmour.com",
        "reebok.com",
        "patagonia.com",
        "alitalia.com",
        "trenitalia.com",
        "volagratis.com",
        "blablacar.it",
        "flixbus.it",
        "c-and-a.com",
        "promod.com",
        "guess.com",
        "mangooutlet.com",
        "citroen.it",
        "opel.it",
        "volvo.it",
        "subaru.it",
        "mitsubishi-motors.it",
        "xiaomi.com",
        "oppo.com",
        "huawei.com",
        "nokia.com",
        "tnt.com",
        "hermes.com",
        "brt.it",
        "social.quandoo.com",
        "coin.it",
        "calzedonia.com",
        "tezenis.com",
        "benetton.com",
        "oysho.com",
        "desigual.com",
        "todis.it",
        "carrefour.it",
        "iper.it",
        "supermedia.it",
        "msi.it",
        "razer.com",
        "corsair.com",
        "logitech.com",
        "vodafone.it",
        "tim.it",
        "windtre.it",
        "fastweb.it",
        "autogrill.it",
        "granarolo.it",
        "barilla.com",
        "mulino.it",
        "ministerosalute.it",
        "acquedotti.it",
        "poltronesofa.com",
        "mediatek.com",
        "qualcomm.com",
        "intel.com",
        "amd.com",
        "nvidia.com",
        "fendi.com",
        "balenciaga.com",
        "versace.com",
        "saintlaurent.com",
        "burberry.com",
        "hermes.com",
        "cartier.com",
        "chanel.com",
        "fisioterapisti.org",
        "farmacoecura.it",
        "humanitas.it",
        "santagostino.it",
        "ducati.com",
        "piaggio.com",
        "iveco.com",
        "bancaintesa.it",
        "unicreditgroup.eu",
        "findomestic.it",
        "fineco.it",
        "n26.com",
        "justeat.it",
        "glovoapp.com",
        "deliveroo.it",
        "ubereats.com",
    }

    // Controlla se il dominio contiene uno dei social media
    for _, smDomain := range socialMediaDomains {
        if strings.Contains(domain, smDomain) {
            fmt.Printf("Sito escluso: %s\n", url)
            return true // Se il dominio è tra quelli esclusi, restituisci true
        }
    }

    // Aggiungi il controllo per i domini che contengono la parola "comune" o "e-coop.it"
    if strings.Contains(domain, "comune.") || strings.Contains(domain, "e-coop.it") {
        fmt.Printf("Sito escluso (comune o e-coop.it): %s\n", url)
        return true // Se il dominio contiene "comune" o "e-coop.it", escludi il sito
    }

    // Aggiungi il controllo per i domini che iniziano con "lecce" o "centrocommerciale"
    if strings.HasPrefix(strings.ToLower(domain), "lecce") || strings.HasPrefix(strings.ToLower(domain), "centrocommerciale") {
        fmt.Printf("Sito escluso (inizia con lecce o centrocommerciale): %s\n", url)
        return true // Se il dominio inizia con "lecce" o "centrocommerciale", escludi il sito
    }

    // Aggiungi il controllo per i domini con estensione .edu.it
    eduAndNetDomains := []string{
        ".edu.it",
    }

    for _, suffix := range eduAndNetDomains {
        if strings.HasSuffix(domain, suffix) {
            fmt.Printf("Sito escluso (.edu.it): %s\n", url)
            return true // Se il dominio ha l'estensione .edu.it, escludi il sito
        }
    }

    // Controlla se il dominio è tra quelli nel file di esclusione
    _, found := excludedWebsites[domain]
    return found
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