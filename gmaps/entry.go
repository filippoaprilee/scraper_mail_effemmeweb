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
    "bufio"

	
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
    // Rimuovi prefissi specifici da ns1 a ns100, dns1 a dns100, ns-cloud- e awsdns-
    for i := 1; i <= 100; i++ {
        prefixes := []string{
            fmt.Sprintf("dns%d.", i),
            fmt.Sprintf("ns%d.", i),
            fmt.Sprintf("ns-cloud-%d.", i),
            fmt.Sprintf("awsdns-%d.", i),
        }

        for _, prefix := range prefixes {
            if strings.HasPrefix(nameserver, prefix) {
                nameserver = strings.TrimPrefix(nameserver, prefix)
                break
            }
        }
    }

    // Rimuovi eventuali suffissi come `.com.`, `.net.`, o il carattere finale `.`
    nameserver = strings.TrimSuffix(nameserver, ".")
    if strings.Contains(nameserver, ".") {
        parts := strings.Split(nameserver, ".")
        if len(parts) > 1 {
            nameserver = strings.Join(parts[:len(parts)-1], ".")
        }
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
    "isiline.it": "Isiline",
    "webme.it": "WebMe",
    "nexcess.net": "NexCess & LiquidWeb",
    "myprivatehosting.biz": "ServerEasy",
    "anycast.me": "OVH Anycast",
    "hostnuoviclienti.com": "Keliweb & NuoviClienti",
    "keliweb.it": "Keliweb",
    "hostnuoviclienti.org": "Keliweb & NuoviClienti",
    "hostnuoviclienti.net": "Keliweb & NuoviClienti",
    "aziendeitalia.it": "Aziende Italia",
    "aziendeitalia.com": "Aziende Italia",
    "aziendeitalia.cz": "Aziende Italia",
    "infinitynet.it": "Infinity Net",
    "bookmyname.com": "Book My Name",
    "momit.eu": "Momit",
    "noamweb.eu": "NoamWeb",
    "noamweb.net": "NoamWeb",
    "sintenet.net": "Sinenet",
    "seeoux.com": "Seeoux",
    "utixo.eu": "Utixo",
    "utixo.net": "Utixo",
    "erweb.it": "ErWeb",
    "serverlet.it": "Serverlet",
    "serverlet.com": "Serverlet",
    "ergonet-dns.it": "Ergonet",
    "ergonet-dns.com": "Ergonet",
    "levita-dns.it": "Levita",
    "levita-dns.eu": "Levita",
    "60gea.com": "60Gea (Enom)",
    "welcomeitalia.it": "ViaNova",
    "mvmnet.com": "MovieMent",
    "mvmnet.it": "MovieMent",
    "mvmnet-dns.eu": "MovieMent",
    "gandi.net": "Gandi.Net",
    "a.gandi.net": "Gandi.Net",
    "b.gandi.net": "Gandi.Net",
    "protone.dns.tiscali.it": "Tiscali",
    "elettrone.dns.tiscali.it": "Tiscali",
    "gidinet.it": "GiDiNet",
    "gidinet.com": "GiDiNet",
    "serverwl.it": "ServerWL",
    "rockethosting.it": "Rocket Hosting",
    "cloudonthecloud.com": "Cloud on The Cloud",
    "fvscloud.it": "OVH",
    "fvscloud.com": "OVH",
    "kpnqwest.it": "Retelit",
    "retelit.it": "Retelit",
    "si-tek.it": "Si-Tek",
    "si-tek.net": "Si-Tek",

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
    maxRetries := 3             // Numero massimo di tentativi
    retryDelay := 5 * time.Second // Ritardo tra un tentativo e l'altro
    timeout := 15 * time.Second   // Timeout per ogni richiesta

    var lastError error

    for attempt := 1; attempt <= maxRetries; attempt++ {
        client := &http.Client{
            Timeout: timeout,
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= 5 {
                    return http.ErrUseLastResponse
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
            time.Sleep(retryDelay)
            continue
        }
        defer resp.Body.Close()

        // Gestione dei codici di stato specifici
        switch resp.StatusCode {
        case http.StatusOK: // 200
            return "Sì - Disponibile (200 OK)", nil
        case http.StatusNonAuthoritativeInfo: // 203
            return "Sì - Informazioni Non Autorevoli (203)", nil
        case http.StatusUnauthorized: // 401
            return "No - Non Autorizzato (401)", nil
        case http.StatusNotFound: // 404
            return "No - Pagina Non Trovata (404)", nil
        case http.StatusSeeOther: // 303
            return "Sì - Vedi Altro (303)", nil
        case http.StatusNoContent: // 204
            return "Sì - Nessun Contenuto (204)", nil
        case http.StatusInternalServerError: // 500
            return "No - Errore Interno del Server (500)", nil
        case http.StatusMovedPermanently: // 301
            return "Sì - Spostato Permanentemente (301)", nil
        case http.StatusBadRequest: // 400
            return "No - Richiesta Errata (400)", nil
        case http.StatusForbidden: // 403
            return "No - Accesso Negato (403)", nil
        case http.StatusNotImplemented: // 501
            return "No - Funzionalità Non Implementata (501)", nil
        case http.StatusBadGateway: // 502
            return "No - Gateway Non Valido (502)", nil
        default:
            if resp.StatusCode >= 400 && resp.StatusCode < 500 {
                return fmt.Sprintf("No - Errore Client (%d)", resp.StatusCode), nil
            } else if resp.StatusCode >= 500 {
                return fmt.Sprintf("No - Errore Server (%d)", resp.StatusCode), nil
            }
        }

        return "No - Stato Non Riconosciuto", nil
    }

    // Se tutti i tentativi falliscono, restituisci l'ultimo errore
    if lastError != nil {
        return "No - Errore dopo vari tentativi", lastError
    }

    return "No - Stato Non Determinato", nil
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
			return "Sì"
		}
	}

	// Cerca nel tag <title>
	re := regexp.MustCompile(`<title>(.*?)<\/title>`)
	match := re.FindStringSubmatch(strings.ToLower(html))
	if len(match) > 1 {
		title := match[1]
		for _, keyword := range maintenanceKeywords {
			if strings.Contains(title, keyword) {
				return "Sì"
			}
		}
	}

	// Cerca anche nel tag <body> per altre parole chiave
	reBody := regexp.MustCompile(`<body.*?>(.*?)<\/body>`)
	bodyMatch := reBody.FindStringSubmatch(html)
	if len(bodyMatch) > 1 {
		bodyContent := bodyMatch[1]
		for _, keyword := range maintenanceKeywords {
			if strings.Contains(strings.ToLower(bodyContent), keyword) {
				return "Sì"
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

    // Verifica se il sito web è escluso o è un social media
    dominio := e.WebSite
    if dominio == "" || isExcludedWebsite(dominio, excludedWebsites) || isSocialMediaDomain(dominio) {
        dominio = ""  // Se il sito è escluso o appartiene ai social media, azzera il campo
    }

    // Aggiungi il provider di hosting solo se il dominio non è escluso
    hostingProvider := e.HostingProvider
    if dominio != "" {
        hostingProvider, _ = getHostingProvider(dominio)  // Chiamata per ottenere il provider
    }

    return []string{
        e.Title,
        e.Category,
        dominio,  // Se il sito è escluso, il campo viene lasciato vuoto
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
        "ikea.",
        "mcdonalds.",
        "apple.",
        "coop.",
        "conad.",
        "esselunga.",
        "lidl.",
        "carrefour.",
        "kfc.",
        "burgerking.",
        "subway.",
        "starbucks.",
        "zara.",
        "nike.",
        "adidas.",
        "primark.",
        "mondoconv.",
        "kasanova.",
        "maisonsdumonde.",
        "euronics.",
        "mediaworld.",
        "decathlon.",
        "leroymerlin.",
        "zalando.",
        "unieuro.",
        "feltrinelli.",
        "gamestop.",
        "auchan.",
        "despar.",
        "pam.",
        "eurospin.",
        "selexgc.",
        "iper.",
        "bennet.",
        "mdspa.",
        "pennymarket.",
        "tigros.",
        "famila.",
        "ilgigante.",
        "aliper.",
        "dok.",
        "sisa.",
        "todis.",
        "simplymarket.",
        "insmercato.",
        "maxidi.",
        "leroymerlin.",
        "chanel.",
        "gucci.",
        "louisvuitton.",
        "prada.",
        "versace.",
        "dior.",
        "hermes.",
        "balenciaga.",
        "ferragamo.",
        "burberry.",
        "boss.",
        "armani.",
        "ralphlauren.",
        "calvinklein.",
        "tommyhilfiger.",
        "hm.",
        "uniqlo.",
        "gap.",
        "bershka.",
        "stradivarius.",
        "pullandbear.",
        "mango.",
        "shein.",
        "northface.",
        "patagonia.",
        "columbia.",
        "reebok.",
        "puma.",
        "underarmour.",
        "newbalance.",
        "converse.",
        "vans.",
        "admiral.",
        "asics.",
        "timberland.",
        "decathlon.",
        "spalding.",
        "nike.",
        "diesel.",
        "levi.",
        "wrangler.",
        "lee.",
        "dockers.",
        "replayjeans.",
        "pepejeans.",
        "tommy.",
        "zegna.",
        "boss.",
        "lacoste.",
        "hogan.",
        "tods.",
        "valentino.",
        "bulgari.",
        "cartier.",
        "piaget.",
        "longines.",
        "rolex.",
        "tudorwatch.",
        "omgega.",
        "tagheuer.",
        "hublot.",
        "longines.",
        "panerai.",
        "jagerlecoultre.",
        "vacheronconstantin.",
        "audemarspiguet.",
        "patek.",
        "girard-perregaux.",
        "zegna.",
        "armani.",
        "gucci.",
        "versace.",
        "michaelkors.",
        "tiffany.",
        "pandora.",
        "swarovski.",
        "bluespirit.",
        "nava.",
        "kitchenmarket.",
        "alessiofurniture.",
        "maisonsdumonde.",
        "franke.",
        "smeg.",
        "whirlpool.",
        "electrolux.",
        "philips.",
        "dyson.",
        "rowenta.",
        "polti.",
        "kenwood.",
        "bialetti.",
        "delonghi.",
        "krups.",
        "espresso.",
        "saeco.",
        "nivona.",
        "nespresso.",
        "caffitaly.",
        "illy.",
        "lavazza.",
        "kimbo.",
        "bondi.",
        "trony.",
        "euronics.",
        "expert.",
        "mediamarket.",
        "unieuro.",
        "mymarket.",
        "penny.",
        "pamlocal.",
        "desparlocal.",
        "auchanlocal.",
        "coinmarket.",
        "cooplocal.",
        "deltaairlines.",
        "ryanair.",
        "iberia.",
        "vueling.",
        "easyjet.",
        "airfrance.",
        "lufthansa.",
        "swissair.",
        "alitalia.",
        "ita.",
        "emirates.",
        "etihad.",
        "qatarairways.",
        "aircanada.",
        "delta.",
        "americanairlines.",
        "united.",
        "britishairways.",
        "koreanair.",
        "thaiairways.",
        "singaporeair.",
        "malaysiaairlines.",
        "cathaypacific.",
        "hainan.",
        "hongkongair.",
        "ana.",
        "jejuair.",
        "virgin.",
        "kidkaboose.",
        "samsung.",
        "lg.",
        "sony.",
        "panasonic.",
        "sharp.",
        "vizio.",
        "hisense.",
        "tcl.",
        "spectrum.",
        "xfinity.",
        "directv.",
        "dish.",
        "netflix.",
        "hulu.",
        "amazon.",
        "ebay.",
        "aliexpress.",
        "shopify.",
        "etsy.",
        "walmart.",
        "target.",
        "bestbuy.",
        "homedepot.",
        "lowes.",
        "menards.",
        "sears.",
        "kohls.",
        "macy.",
        "nordstrom.",
        "bloomingdales.",
        "costco.",
        "samsclub.",
        "bjs.",
        "guitarcenter.",
        "music123.",
        "sweetwater.",
        "musiciansfriend.",
        "reverb.",
        "azmusic.",
        "zsounds.",
        "prosoundgear.",
        "professormics.",
        "audio-technica.",
        "bose.",
        "jbl.",
        "sennheiser.",
        "shure.",
        "koss.",
        "sony.",
        "pioneer.",
        "yamaha.",
        "roland.",
        "korg.",
        "zoom.",
        "mackie.",
        "alesis.",
        "behringer.",
        "digidesign.",
        "focusrite.",
        "steinberg.",
        "songkick.",
        "livemusic.",
        "concertpass.",
        "ticketmaster.",
        "stubhub.",
        "seatgeek.",
        "eventbrite.",
        "axs.",
        "ticketsnow.",
        "viagogo.",
        "tix.",
        "tickpick.",
        "vividseats.",
        "livenation.",
        "fandango.",
        "flixster.",
        "rottentomatoes.",
        "metacritic.",
        "imdb.",
        "amctheatres.",
        "regmovies.",
        "cinemark.",
        "cineplex.",
        "showcasecinemas.",
        "alamodrafthouse.",
        "drafthouse.",
        "harkins.",
        "angelikafilmcenter.",
        "landmarktheatres.",
        "eastlandcinema.",
        "cinema.",
        "focusfeatures.",
        "universalmovies.",
        "paramountplus.",
        "disneyplus.",
        "hbo.",
        "hulu.",
        "showtime.",
        "starz.",
        "cinemax.",
        "crackle.",
        "tubi.",
        "plutotv.",
        "peacocktv.",
        "youtube.",
        "vimeo.",
        "dailymotion.",
        "snapchat.",
        "tiktok.",
        "twitter.",
        "pinterest.",
        "tumblr.",
        "reddit.",
        "quora.",
        "medium.",
        "wordpress.",
        "blogger.",
        "wix.",
        "squarespace.",
        "weebly.",
        "shopify.",
        "bigcommerce.",
        "magento.",
        "woocommerce.",
        "prestashop.",
        "opencart.",
        "volusion.",
        "3dcart.",
        "squareup.",
        "paypal.",
        "stripe.",
        "worldpay.",
        "adyen.",
        "authorizenet.",
        "skrill.",
        "payoneer.",
        "amazonpay.",
        "googlepay.",
        "applepay.",
        "samsungpay.",
        "cryptopayments.",
        "coinbase.",
        "binance.",
        "kraken.",
        "bitpay.",
        "blockchain.",
        "gemini.",
        "poloniex.",
        "bitstamp.",
        "bithumb.",
        "okx.",
        "kucoin.",
        "gate.",
        "crypto.",
        "luno.",
        "bitfinex.",
        "celsius.",
        "nexo.",
        "blockfi.",
        "hodlhodl.",
        "paxful.",
        "localbitcoins.",
        "cashapp.",
        "venmo.",
        "zellepay.",
        "wise.",
        "revolut.",
        "monzo.",
        "n26.",
        "chime.",
        "sofi.",
        "varomoney.",
        "ally.",
        "capitalone.",
        "discover.",
        "americanexpress.",
        "citibank.",
        "wellsfargo.",
        "jpmorganchase.",
        "bankofamerica.",
        "hsbc.",
        "barclays.",
        "santander.",
        "unicredit.",
        "intesasanpaolo.",
        "bnpparibas.",
        "societegenerale.",
        "creditagricole.",
        "deutschebank.",
        "commerzbank.",
        "abnamro.",
        "rabobank.",
        "ubs.",
        "credit-suisse.",
        "natwest.",
        "lloydsbank.",
        "standardchartered.",
        "commonwealthbank.",
        "westpac.",
        "ocbc.",
        "bankofchina.",
        "icbc.",
        "ccb.",
        "agriculturebank.",
        "bankofindia.",
        "statebankofindia.",
        "hdfcbank.",
        "axisbank.",
        "icicibank.",
        "kotak.",
        "idbibank.",
        "punjabnationalbank.",
        "bankofbaroda.",
        "yesbank.",
        "indusindbank.",
        "unionbank.",
        "canarabank.",
        "centralbank.",
        "dhanbank.",
        "dentalpro.",
        "sorrisiesalute.",
        "dentix.",
        "implantologiadentale.",
        "dentistiitaliani.",
        "mydentist.",
        "studidentisticiitalia.",
        "dentista.",
        "dentalcoop.",
        "dentaltree.",
        "centrodentale.",
        "sandental.",
        "artedent.",
        "orisdental.",
        "ciromandelli.",
        "smileclinic.",
        "dentaleuropeo.",
        "dentafutura.",
        "centroodontoiatricofutura.",
        "dentalone.",
        "mirvisrl.",
        "dentalgroupitalia.",
        "studiodentisticogroup.",
        "dentalnet.",
        "dentalclinics.",
        "dentalfamily.",
        "dentalcity.",
        "dentalitalia.",
        "bioclinicitalia.",
        "dentistapiu.",
        "dentalhouseitalia.",
        "dentalserviceitalia.",
        "dentalspaitalia.",
        "biomeditalia.",
        "dentalitalgroup.",
        "dentalclinicit.",
        "dentistsrl.",
        "sorridepiu.",
        "dentart.",
        "dentalmedic.",
        "dentstudio.",
        "sanitadental.",
        "dentalgroupitalia.",
        "dentalpoint.",
        "grandvision.",
        "salmoiraghievigano.",
        "otticaveneta.",
        "visionottica.",
        "otticavistar.",
        "occhialihouse.",
        "occhialionline.",
        "otticadentale.",
        "otticaonline.",
        "otticamoderna.",
        "occhialidelsole.",
        "otticalux.",
        "otticacorrado.",
        "otticapiu.",
        "occhialland.",
        "otticacentrovisione.",
        "otticalounge.",
        "otticapoint.",
        "sunsightitalia.",
        "luxottica.",
        "ovsoptics.",
        "lentispeciali.",
        "occhiali4you.",
        "occhialishop.",
        "occhialarte.",
        "ovisual.",
        "specchionline.",
        "vistaitalia.",
        "otticafutura.",
        "otticadelcorso.",
        "vistafacile.",
        "glamouroptics.",
        "otticadigitale.",
        "otticacentral.",
        "otticadelcentro.",
        "otticasole.",
        "ginecologiapro.",
        "centroginecologia.",
        "studioginecologico.",
        "ginecologialife.",
        "femclinic.",
        "ginecologiapiu.",
        "centrodiagnosticoginecologico.",
        "ladonnaesalute.",
        "mediciginecologi.",
        "progettoginecologia.",
        "ginecologiadonna.",
        "studiogin.",
        "diagnosticaginecologica.",
        "centroginecologiaprofessionale.",
        "studiomedicoginecologico.",
        "saluteginecologica.",
        "ginecologialuce.",
        "gineclinic.",
        "ginecare.",
        "ginemed.",
        "medicinadonna.",
        "clinicamaterna.",
        "gineclinicpro.",
        "medgyn.",
        "profgynecology.",
        "studiogynlife.",
        "gynplus.",
        "gynfamily.",
        "centromaternal.",
        "clinicaginelife.",
        "istruzione.",
        "miur.gov.",
        "sedonline.",
        "scuolaweb.",
        "portaledellascuola.",
        "scuoleitalia.",
        "universitalia.",
        "accademiadellearti.",
        "institutodidattico.",
        "campusitalia.",
        "superiorieducation.",
        "masterstudies.",
        "liceitaliani.",
        "istruzioneeformazione.",
        "scuolaitaliana.",
        "informazionescolastica.",
        "studiuniversitari.",
        "universitalian.",
        "istruzionesuperiore.",
        "miurscuole.",
        "sistemaistruzione.",
        "scuolapubblica.",
        "accademieartistiche.",
        "formazionetecnica.",
        "corsieducativi.",
        "studitalia.",
        "insegnamentoitalia.",
        "scolasticaonline.",
        "sistemadidattico.",
        "educazioneprofessionale.",
        "centrodistudi.",
        "studioitaliano.",
        "scuoleonlines.",
        "institutionsedu.",
        "studioselearning.",
        "scuoleformazione.",
        "scuoladigital.",
        "aslroma1.",
        "aslroma2.",
        "aslroma3.",
        "aslroma4.",
        "aslroma5.",
        "aslroma6.",
        "aslviterbo.",
        "aslrieti.",
        "asllatina.",
        "aslfrosinone.",
        "asltorino.",
        "aslcuneo.",
        "unes.",
        "dentista.tv",
        "aslbiella.",
        "aslalessandria.",
        "aslasti.",
        "aslnovara.",
        "aslvercelli.",
        "aslverbania.",
        "asltorino3.",
        "aslto4.",
        "aslto5.",
        "aslcitta.",
        "aslbergamo.",
        "aslbrescia.",
        "aslcomo.",
        "aslcremona.",
        "asllecco.",
        "asllodi.",
        "aslmantova.",
        "aslmilano.",
        "aslmonza.",
        "aslpavia.",
        "aslsondrio.",
        "aslvarese.",
        "aslvenezia.",
        "aslpadova.",
        "asltreviso.",
        "aslvicenza.",
        "aslverona.",
        "aslrovigo.",
        "aslbelluno.",
        "asltrieste.",
        "aslgorizia.",
        "asludine.",
        "aslpordenone.",
        "aslgenova.",
        "aslsavona.",
        "aslimperia.",
        "aslspezia.",
        "aslparma.",
        "aslmodena.",
        "aslreggioemilia.",
        "aslferrara.",
        "aslravennail.",
        "aslbologna.",
        "aslforli.",
        "aslcesena.",
        "aslrimini.",
        "aslpisa.",
        "aslfirenze.",
        "aslprato.",
        "aslarezzo.",
        "asllivorno.",
        "aslsiena.",
        "aslgrosseto.",
        "aslmassa.",
        "asllatina.",
        "asllucca.",
        "aslpistoia.",
        "asltoscana.",
        "aslterni.",
        "aslperugia.",
        "aslmarche.",
        "aslancona.",
        "aslfano.",
        "aslascoli.",
        "aslpesaro.",
        "aslfoggia.",
        "aslbarletta.",
        "aslbrindisi.",
        "asllecce.",
        "asltaranto.",
        "aslbari.",
        "aslpotenza.",
        "aslmatera.",
        "aslcatanzaro.",
        "aslreggiocalabria.",
        "aslcosenza.",
        "aslcrotone.",
        "aslvibovalentia.",
        "aslagr.",
        "aslsiracusa.",
        "aslcatania.",
        "aslragusa.",
        "asltrapani.",
        "aslenna.",
        "aslcaltanissetta.",
        "aslpalermo.",
        "aslprato.",
        "aslsassari.",
        "aslcagliari.",
        "edilportale.",
        "edilizialavoro.",
        "cantiereonline.",
        "ediliziaeprogetti.",
        "materialiedili.",
        "supersiti.",
        "costruttoriitaliani.",
        "progettoedilizia.",
        "ediliziaweb.",
        "buildingsmartitalia.",
        "architettiitaliani.",
        "ediliziasostenibile.",
        "smartbuildingitalia.",
        "casainnovativa.",
        "sistemiedilizi.",
        "tecnologiedilizia.",
        "ingegneriacivile.",
        "ediliziamoderna.",
        "ristrutturazioneitalia.",
        "edificiuniti.",
        "ediliziabuilding.",
        "casaedilizia.",
        "ediliziapiu.",
        "edilglobal.",
        "areacostruzioni.",
        "mattoniedilizia.",
        "ediliziafacile.",
        "edilsite.",
        "ristrutturaora.",
        "soluzioniedili.",
        "edilizialine.",
        "italcostruzioni.",
        "ediltrend.",
        "innovazioneedile.",
        "ediliziainrete.",
        "bigconstruction.",
        "italianedilizia.",
        "tuttoperedilizia.",
        "ediliziacentro.",
        "networkedilizia.",
        "casarinnovata.",
        "zara.",
        "hm.",
        "uniqlo.",
        "bershka.",
        "stradivarius.",
        "pullandbear.",
        "mango.",
        "shein.",
        "northface.",
        "patagonia.",
        "columbia.",
        "levi.",
        "wrangler.",
        "lee.",
        "diesel.",
        "timberland.",
        "replayjeans.",
        "pepejeans.",
        "sisley.",
        "benetton.",
        "calvinklein.",
        "tommyhilfiger.",
        "boss.",
        "armani.",
        "gucci.",
        "versace.",
        "louisvuitton.",
        "prada.",
        "michaelkors.",
        "valentino.",
        "chanel.",
        "dior.",
        "balenciaga.",
        "ferragamo.",
        "tods.",
        "hogan.",
        "lacoste.",
        "ralphlauren.",
        "nike.",
        "puma.",
        "adidas.",
        "underarmour.",
        "newbalance.",
        "converse.",
        "vans.",
        "oxbow.",
        "carhartt.",
        "superdry.",
        "quiksilver.",
        "billabong.",
        "oakley.",
        "ripcurl.",
        "dcshoes.",
        "elementbrand.",
        "rvca.",
        "volcom.",
        "hurley.",
        "roxy.",
        "americaeagle.",
        "oldnavy.",
        "abercrombie.",
        "hollisterco.",
        "burberry.",
        "massimodutti.",
        "thekooples.",
        "zadig-et-voltaire.",
        "sandro-paris.",
        "maje.",
        "bcbg.",
        "anthropologie.",
        "freepeople.",
        "nordstrom.",
        "macy.",
        "kohls.",
        "bloomingdales.",
        "saksfifthavenue.",
        "selfridges.",
        "harrods.",
        "debenhams.",
        "zalando.",
        "boohoo.",
        "asos.",
        "myntra.",
        "farfetch.",
        "net-a-porter.",
        "ssense.",
        "shopbop.",
        "endclothing.",
        "nordstromrack.",
        "backcountry.",
        "mrporter.",
        "bluefly.",
        "gilt.",
        "oui.",
        "modaoperandi.",
        "italist.",
        "yoox.",
        "lanecrawford.",
        "flannels.",
        "revolve.",
        "discountuniverse.",
        "maisonmargiela.",
        "alexanderwang.",
        "proenzaschouler.",
        "alexandermcqueen.",
        "marcjacobs.",
        "katespade.",
        "toryburch.",
        "coach.",
        "dkny.",
        "moschino.",
        "louboutin.",
        "manoloblahnik.",
        "miu-miu.",
        "viviennewestwood.",
        "stella-mccartney.",
        "karl.",
        "pinko.",
        "maxmara.",
        "twinset.",
        "liujo.",
        "ysl.",
        "ermenegildozegna.",
        "bally.",
        "fendi.",
        "tods.",
        "moreschi.",
        "canali.",
        "kiton.",
        "brunellocucinelli.",
        "boglioli.",
        "lardini.",
        "incotex.",
        "ptpantaloni.",
        "tombolini.",
        "barbour.",
        "belstaff.",
        "gloverall.",
        "moncler.",
        "napapijri.",
        "cpcompany.",
        "schutz.",
        "aldo.",
        "stevemadden.",
        "clarks.",
        "birkenstock.",
        "crocs.",
        "famousfootwear.",
        "dsw.",
        "zappos.",
        "shoemall.",
        "onlineshoes.",
        "shoecarnival.",
        "footlocker.",
        "finishline.",
        "dickssportinggoods.",
        "sportsdirect.",
        "athleta.",
        "gymshark.",
        "reebok.",
        "asics.",
        "champion.",
        "brooksrunning.",
        "hoka.",
        "sketchers.",
        "kappa.",
        "umbro.",
        "columbiasportswear.",
        "campion.",
        "madewell.",
        "bodenusa.",
        "talbots.",
        "loft.",
        "express.",
        "francescas.",
        "forever21.",
        "romwe.",
        "zaful.",
        "prettyfashion.",
        "everlane.",
        "outdoorvoices.",
        "nikefactory.",
        "reebokoutlet.",
        "asicsoutlet.",
        "championoutlet.",
        "hanes.",
        "carters.",
        "kidsfashion.",
        "hmchildrens.",
        "babyshop.",
        "mothercare.",
        "kidsworld.",
        "childrensplace.",
        "toysclothing.",
        "jojomamanbebe.",
        "mamasandpapas.",
        "kidswearhouse.",
        "laredoute.",
        "matalan.",
        "george.",
        "dorothyperkins.",
        "wallisfashion.",
        "peacocks.",
        "newlook.",
        "riverisland.",
        "jackwills.",
        "superdry.",
        "tedbaker.",
        "houseoffraser.",
        "dune.",
        "oasisfashion.",
        "warehousefashion.",
        "karenmillen.",
        "coastfashion.",
        "burton.",
        "next.",
        "primark.",
        "debenhams.",
        "johnlewis.",
        "marksandspencer.",
        "clothingattesco.",
        "fandf.",
        "halfords.",
        "directsports.",
        "sportdirectuk.",
        "hotter.",
        "shoezone.",
        "barratts.",
        "officersclub.",
        "slaters.",
        "scottsmenswear.",
        "usc.",
        "fashionworld.",
        "longtallsally.",
        "gapoutlet.",
        "oldnavyoutlet.",
        "mensfashionwear.",
        "menswearhouse.",
        "josbank.",
        "bigandtailor.",
        "xlclothing.",
        "mensfashionxl.",
        "largefashion.",
        "pediatriaprofessionale.",
        "centropediatria.",
        "studiopediatrico.",
        "pediatricapiu.",
        "pediatriainfantile.",
        "famigliapediatrica.",
        "pediatricamedica.",
        "salutepediatrica.",
        "pediatriaonline.",
        "pediatricgroup.",
        "progettosalutepediatrica.",
        "pediatricafutura.",
        "pediatriainrete.",
        "centroinfanzia.",
        "bambinisalute.",
        "pediatrianetwork.",
        "pediatricoop.",
        "pediatrimed.",
        "pediatriclinic.",
        "pediatriacentral.",
        "pediatriadelbambino.",
        "pediatrianova.",
        "clinicadelbambino.",
        "pediatriaitaliana.",
        "pediatriadelbenessere.",
        "pediatriasanitaria.",
        "ospedalepediatrico.",
        "pediatriaregione.",
        "pediatriafamily.",
        "pediatricchild.",
        "pediatricarete.",
        "pediatrimedicale.",
        "pediatriadellasalute.",
        "pediatriafacile.",
        "pediatricpoint.",
        "pediatriacura.",
        "pediatrialight.",
        "centropediatricasalute.",
        "bmw.",
        "audibyl.",
        "mercedes-benz.",
        "vw.",
        "porsche.",
        "toyota.",
        "lexus.",
        "nissanusa.",
        "hyundai.",
        "kia.",
        "honda.",
        "mazda.",
        "subaru.",
        "suzuki.",
        "mitsubishi-motors.",
        "volvo.",
        "landrover.",
        "jaguar.",
        "fiat.",
        "alfa-romeo.",
        "lancia.",
        "ferrari.",
        "lamborghini.",
        "maserati.",
        "bentleymotors.",
        "rolls-roycemotorcars.",
        "bugatti.",
        "ashtonmartin.",
        "mcLaren.",
        "dacia.",
        "peugeot.",
        "citroen.",
        "opel.",
        "renault.",
        "skoda-auto.",
        "seat.",
        "chevrolet.",
        "ford.",
        "dodge.",
        "jeep.",
        "chrysler.",
        "buick.",
        "gmc.",
        "tesla.",
        "lucidmotors.",
        "dm-drogeriemarkt.",
        "rivian.",
        "polestar.",
        "byd.",
        "geely.",
        "greatwall.",
        "mahindra.",
        "tata.",
        "mvagusta.",
        "kawasakimotorcycle.",
        "sherco.",
        "ducati.",
        "harley-davidson.",
        "swm.",
        "divinci.",
        "suzuki.",
        "ford.",
        "alfa.",
        "fiat.",
        "skoda.",
        "volkswagen.",
        "bentley.",
        "hummer.",
        "yugo.",
        "saaab.",
        "bsa.",
        "morini.",
        "bmwitalia.",
        "daciaitalia.",
        "opelitalia.",
        "jeepitalia.",
        "forditalia.",
        "teslamotors.",
        "nio.",
        "karson.",
        "volocopter.",
        "mobilissimo.",
        "semtex.",
        "bmw.",
        "audibyl.",
        "mercedes-benz.",
        "vw.",
        "porsche.",
        "toyota.",
        "lexus.",
        "nissanusa.",
        "hyundai.",
        "kia.",
        "honda.",
        "mazda.",
        "subaru.",
        "suzuki.",
        "mitsubishi-motors.",
        "volvo.",
        "landrover.",
        "jaguar.",
        "fiat.",
        "alfa-romeo.",
        "lancia.",
        "ferrari.",
        "lamborghini.",
        "maserati.",
        "bentleymotors.",
        "rolls-roycemotorcars.",
        "bugatti.",
        "ashtonmartin.",
        "mcLaren.",
        "dacia.",
        "peugeot.",
        "citroen.",
        "opel.",
        "renault.",
        "skoda-auto.",
        "seat.",
        "chevrolet.",
        "ford.",
        "dodge.",
        "jeep.",
        "chrysler.",
        "buick.",
        "gmc.",
        "tesla.",
        "lucidmotors.",
        "rivian.",
        "polestar.",
        "byd.",
        "geely.",
        "greatwall.",
        "mahindra.",
        "tata.",
        "mvagusta.",
        "kawasakimotorcycle.",
        "sherco.",
        "ducati.",
        "harley-davidson.",
        "swm.",
        "divinci.",
        "suzuki.",
        "ford.",
        "alfa.",
        "fiat.",
        "skoda.",
        "volkswagen.",
        "bentley.",
        "hummer.",
        "yugo.",
        "saaab.",
        "bsa.",
        "morini.",
        "bmwitalia.",
        "daciaitalia.",
        "opelitalia.",
        "jeepitalia.",
        "forditalia.",
        "teslamotors.",
        "nio.",
        "karson.",
        "volocopter.",
        "mobilissimo.",
        "semtex.",
        "harley-davidson.",
        "indianmotorcycle.",
        "yamaha-motor.",
        "kawasaki.",
        "suzuki-motorcycle.",
        "honda-motorcycle.",
        "ducati.",
        "triumphmotorcycles.",
        "bmw-motorrad.",
        "kymco.",
        "aprilia.",
        "piaggio.",
        "moto-guzzi.",
        "vespa.",
        "royalenfield.",
        "beta-uk.",
        "husqvarna-motorcycles.",
        "gasgas.",
        "swm-motorcycles.",
        "sherco.",
        "bimota.",
        "guzzi.",
        "nortonmotorcycles.",
        "cagiva.",
        "agv.",
        "bellhelmets.",
        "shoei.",
        "arai.",
        "schuberth.",
        "ls2helmets.",
        "hjc.",
        "shark-helmets.",
        "mthelmets.",
        "zeus-helmets.",
        "scorpion-exo.",
        "icon1000.",
        "dainese.",
        "alpinestars.",
        "matrimonio.",
        "revitsport.",
        "spidi.",
        "bull-it.",
        "jeanlouisdavid.",
        "pacorabannehair.",
        "aldobarbershop.",
        "salonemilano.",
        "dessange.",
        "toniandguy.",
        "haircoif.",
        "testanmarques.",
        "provostparrucchieri.",
        "contestarockhair.",
        "salonguerrieri.",
        "diegoalonsosalons.",
        "matrixhair.",
        "framesi.",
        "paulmitchell.",
        "wella.",
        "lorealprofessionnel.",
        "schwarzkopf.",
        "tigi.",
        "hairdreams.",
        "ghdhair.",
        "alteregohair.",
        "evosalon.",
        "kemon.",
        "hairboxitalia.",
        "hairfusion.",
        "hairmania.",
        "biotonic.",
        "aldebaran.",
        "fashioncuts.",
        "hairlux.",
        "beautyfactory.",
        "blowdrybar.",
        "salonelite.",
        "hairdesign.",
        "solair.",
        "salonprofessional.",
        "hairhouse.",
        "salonexperience.",
        "salonmodern.",
        "hairkingdom.",
        "parlux.",
        "hairven.",
        "shinehair.",
        "stylehair.",
        "salonclub.",
        "glamhair.",
        "modhair.",
        "coop.",
        "conad.",
        "esselunga.",
        "lidl.",
        "carrefour.",
        "auchan.",
        "despar.",
        "pam.",
        "eurospin.",
        "selexgc.",
        "iper.",
        "bennet.",
        "mdspa.",
        "pennymarket.",
        "tigros.",
        "famila.",
        "ilgigante.",
        "aliper.",
        "dok.",
        "sisa.",
        "todis.",
        "simplymarket.",
        "insmercato.",
        "maxidi.",
        "spazio.conad.",
        "superconti.",
        "coal.",
        "migross.",
        "magazzinogru.",
        "sigma.",
        "supersigma.",
        "cityper.",
        "aliper.",
        "unicomm.",
        "unicoop.",
        "unicoopfirenze.",
        "ipercoop.",
        "migrossspa.",
        "rossettisupermercati.",
        "mercatodimezzogiorno.",
        "superpan.",
        "craiperte.",
        "futura.",
        "mercatoneuno.",
        "superemme.",
        "scelto.",
        "sidis.",
        "vegmarket.",
        "bio.supermercati.",
        "trekbikes.",
        "canyon.",
        "specialized.",
        "scott-sports.",
        "cannondale.",
        "bianchi.",
        "pinarello.",
        "cervelo.",
        "giant-bicycles.",
        "orbea.",
        "fujibikes.",
        "merida-bikes.",
        "khsbicycles.",
        "kona.",
        "santa-cruz.",
        "focus-bikes.",
        "cube.",
        "ridley-bikes.",
        "colnago.",
        "wilier.",
        "haibike.",
        "marinbikes.",
        "nsbikes.",
        "lapierre-bikes.",
        "argon18bike.",
        "velopress.",
        "feltbicycles.",
        "rosebikes.",
        "ghost-bikes.",
        "yeticycles.",
        "salsa.",
        "ibisbikes.",
        "bmc-switzerland.",
        "rockymountainbikes.",
        "ninerbikes.",
        "polygonbikes.",
        "raleigh.",
        "diamondback.",
        "koga.",
        "schwinnbikes.",
        "parlee.",
        "kinesisbikes.",
        "whytebikes.",
        "gt-bicycles.",
        "gazellebikes.",
        "regione.lombardia.",
        "regione.piemonte.",
        "regione.veneto.",
        "regione.liguria.",
        "regione.emiliaromagna.",
        "regione.toscana.",
        "regione.umbria.",
        "regione.marche.",
        "regione.lazio.",
        "regione.abruzzo.",
        "regione.molise.",
        "regione.campania.",
        "regione.puglia.",
        "regione.basilicata.",
        "regione.calabria.",
        "regione.sicilia.",
        "regione.sardegna.",
        "regione.trentinoaltoadige.",
        "regione.friuliveneziagiulia.",
        "regione.valledaosta.",
        "moustachebikes.",
        "terratrike.",
        "ternbicycles.",
        "lynskeyperformance.",
        "surlybikes.",
        "moots.",
        "matrimonio.",
        "bricoman.",
        "tecnomat.",
        "apple.",
        "arcaplanet.",
        "canon.",
        "globo.",
        "burgerking.",
        "maisonsdumonde.",
        "kfc.",
        "hm.",
        "happycasa.",
        "sonnybono.",
        "levis.",
        "meltingpot.",
        "vikingop.",
        "dhl.",
        "gls-italy.",
        "ups.",
        "fedex.",
        "miodottore.",
        "booking.",
        "airbnb.",
        "twitter.",
        "youtube.",
        "pinterest.",
        "tripadvisor.",
        "tiktok.",
        "wix.",
        "trivago.",
        "squarespace.",
        "godaddy.",
        "weebly.",
        "tumblr.",
        ".edu.",
        "calendar.app.google.",
        "linktr.ee",
        "care-dent.",
        "doctolib.",
        "dentego.",
        "dottori.",
        "yellow.local.",
        "larc.",
        "drmax.",

        "sanita.",
        "fb.me.",
        "poste.",
        "bianalisi.",
        "sisal.",
        "eurobet.",
        "betfair.",
        "roulette.",
        "gioco.",
        "scommesse.",
        "casino.",
        "supermercatidok.",
        "mondadoristore.",
        "amazon.",
        "prenatal.",
        "aeo.",
        "toyscenter.",
        "coop.",
        "conad.",
        "visionottica.",
        "amplifon.",
        "grandivision.",
        "wa.me.",
        "ebay.",
        "aliexpress.",
        "zalando.",
        "asos.",
        "kayak.",
        "kayak.",
        ".gov.",
        "sephora.",
        "douglas.",
        "yves-rocher.",
        "tigota.",
        "welinkbuilders.",
        "lidl.",
        "eurospin.",
        "despar.",
        "mdspa.",
        "aw-lab.",
        "footlocker.",
        "telegram.",
        "viber.",
        "shein.",
        "etsy.",
        "wish.",
        "ikea.",
        "leroymerlin.",
        "mediaworld.",
        "unieuro.",
        "trony.",
        "decathlon.",
        "decathlon.",
        "euronics.",
        "pullandbear.",
        "zalandoprive.",
        "stradivarius.",
        "mcdonalds.",
        "alcott.",
        "mongolfieralecce.",
        "auchan.",
        "intimissimi.",
        "bershka.",
        "zara.",
        "snai.",
        "bwin.",
        "starcasino.",
        "leovegas.",
        "betway.",
        "netflix.",
        "disneyplus.",
        "primevideo.",
        "spotify.",
        "expedia.",
        "lastminute.",
        "skyscanner.",
        "italo.",
        "ryanair.",
        "easyjet.",
        "inps.",
        "agenziaentrate.",
        "anagrafe.",
        "ovs.",
        "coursera.",
        "chicco.",
        "casanovadesign.",
        "casaamica.",
        "buffetti.",
        "poste.",
        "lg.",
        "samsung.",
        "bartolini.",
        "fiat.",
        "lancia.",
        "spaziocasa.",
        "mango.",
        "volkswagen.",
        "bmw.",
        "audi.",
        "mercedes-benz.",
        "peugeot.",
        "renault.",
        "toyota.",
        "hyundai.",
        "nissan.",
        "kia.",
        "seat.",
        "skoda-auto.",
        "jeep.",
        "tesla.",
        "maserati.",
        "lamborghini.",
        "ferrari.",
        "dsautomobiles.",
        "uniqlo.",
        "gap.",
        "banana-republic.",
        "topshop.",
        "calvinklein.",
        "tommy.",
        "diesel.",
        "northface.",
        "timberland.",
        "rolex.",
        "napapijri.",
        "lacoste.",
        "poliziadistato.",
        "carabinieri.",
        "visitlecce.",
        "aldi.",
        "penny.",
        "tigerstores.",
        "cooponline.",
        "esselunga.",
        "asus.",
        "lenovo.",
        "acer.",
        "dell.",
        "hp.",
        "nike.",
        "adidas.",
        "underarmour.",
        "reebok.",
        "patagonia.",
        "alitalia.",
        "trenitalia.",
        "volagratis.",
        "blablacar.",
        "flixbus.",
        "c-and-a.",
        "promod.",
        "guess.",
        "mangooutlet.",
        "citroen.",
        "opel.",
        "volvo.",
        "subaru.",
        "mitsubishi-motors.",
        "xiaomi.",
        "oppo.",
        "huawei.",
        "nokia.",
        "tnt.",
        "hermes.",
        "brt.",
        "social.quandoo.",
        "coin.",
        "calzedonia.",
        "tezenis.",
        "benetton.",
        "oysho.",
        "desigual.",
        "todis.",
        "carrefour.",
        "iper.",
        "supermedia.",
        "msi.",
        "razer.",
        "corsair.",
        "logitech.",
        "vodafone.",
        "tim.",
        "windtre.",
        "fastweb.",
        "autogrill.",
        "granarolo.",
        "barilla.",
        "mulino.",
        "ministerosalute.",
        "acquedotti.",
        "poltronesofa.",
        "mediatek.",
        "qualcomm.",
        "intel.",
        "amd.",
        "nvidia.",
        "fendi.",
        "balenciaga.",
        "versace.",
        "saintlaurent.",
        "burberry.",
        "hermes.",
        "cartier.",
        "chanel.",
        "fisioterapisti.",
        "farmacoecura.",
        "humanitas.",
        "santagostino.",
        "ducati.",
        "piaggio.",
        "iveco.",
        "bancaintesa.",
        "unicreditgroup.",
        "findomestic.",
        "fineco.",
        "n26.",
        "justeat.",
        "glovoapp.",
        "deliveroo.",
        "ubereats.",
        "reidcycles.",
        "mediaworld.",
        "euronics.",
        "unieuro.",
        "trony.",
        "saturn.",
        "expert.",
        "eldoradostore.",
        "mediamarkt.",
        "fnac.",
        "melectronics.",
        "interdiscount.",
        "pccity.",
        "gamedigital.",
        "microcenter.",
        "bestbuy.",
        "currys.",
        "apple.",
        "dell.",
        "hp.",
        "asus.",
        "lenovo.",
        "acer.",
        "samsung.",
        "sony.",
        "lg.",
        "huawei.",
        "xiaomi.",
        "oneplus.",
        "oppo.",
        "vivo.",
        "nokia.",
        "motorola.",
        "intel.",
        "amd.",
        "nvidia.",
        "anker.",
        "bose.",
        "jbl.",
        "logitech.",
        "sennheiser.",
        "western-digital.",
        "seagate.",
        "crucial.",
        "corsair.",
        "kingston.",
        "adata.",
        "gigabyte.",
        "msi.",
        "razer.",
        "steelseries.",
        "thrustmaster.",
        "sharkoon.",
        "nzxt.",
        "tp-link.",
        "netgear.",
        "d-link.",
        "asustor.",
        "synology.",
        "qnap.",
        "nest.",
        "gopro.",
        "dji.",
        "ring.",
        "fiat.",
        "intesa.",
        "unicreditgroup.",
        "generali.",
        "luxottica.",
        "telecomitalia.",
        "postitaliane.",
        "leonardo.",
        "ferroviedellostato.",
        "saipem.",
        "italcementi.",
        "prysmian.",
        "fincantieri.",
        "snam.",
        "mediolanum.",
        "benetton.",
        "tods.",
        "moncler.",
        "ferrari.",
        "lamborghini.",
        "bulgari.",
        "chicco.",
        "ducati.",
        "bialetti.",
        "pirelli.",
        "bialettigroup.",
        "amplifon.",
        "cucinelli.",
        "campari.",
        "barilla.",
        "mutuionline.",
        "yoox.",
        "moleskine.",
        "cattolica.",
        "generalmotors.",
        "target.",
        "caterpillar.",
        "google.",
        "amazon.",
        "apple.",
        "microsoft.",
        "ibm.",
        "samsung.",
        "tencent.",
        "alphabet.",
        "siemens.",
        "philips.",
        "disney.",
        "cisco.",
        "pepsico.",
        "cocacola.",
        "netflix.",
        "visa.",
        "mastercard.",
        "paypal.",
        "booking.",
        "adidas.",
        "nike.",
        "uber.",
        "lyft.",
        "airbnb.",
        "glovoapp.",
        "deliveroo.",
        "justeat.",
        "ubereats.",
        "foodora.",
        "domicilios.",
        "pizza-boom.",
        "pizzahutdelivery.",
        "dominos.",
        "postmates.",
        "grubhub.",
        "doordash.",
        "caviar.",
        "seamless.",
        "foodpanda.",
        "zomato.",
        "swiggy.",
        "instacart.",
        "shipt.",
        "getir.",
        "gorillas.",
        "flink.",
        "carrefourdelivery.",
        "easycoop.",
        "spesaalvolo.",
        "picnic.",
        "grocerydelivery.",
        "freshdirect.",
        "box8.",
        "zeekit.",
        "ubereatsdrivers.",
        "courierexpress.",
        "fooddeliveries.",
        "myeats.",
        "ubereatsrider.",
        "deliveryhero.",
        "takeaway.",
        "rocketdelivery.",
        "eatzapp.",
        "bitesquad.",
        "marleyspoon.",
        "hungryroot.",
        "hellofresh.",
        "amazon.",
        "ebay.",
        "aliexpress.",
        "etsy.",
        "walmart.",
        "target.",
        "shein.",
        "zalando.",
        "asos.",
        "farfetch.",
        "net-a-porter.",
        "ssense.",
        "shopify.",
        "depop.",
        "temumarketplace.",
        "cdiscount.",
        "mercadolivre.",
        "flipkart.",
        "myntra.",
        "snapdeal.",
        "rakuten.",
        "uniqlo.",
        "hm.",
        "zara.",
        "pullandbear.",
        "stradivarius.",
        "mango.",
        "nike.",
        "adidas.",
        "reebok.",
        "underarmour.",
        "converse.",
        "vans.",
        "newbalance.",
        "patagonia.",
        "columbia.",
        "theoutnet.",
        "bestbuy.",
        "currys.",
        "mediatechstore.",
        "euronics.",
        "unieuro.",
        "trony.",
        "mediaworld.",
        "apple.",
        "samsung.",
        "xiaomi.",
        "hp.",
        "lenovo.",
        "asus.",
        "dell.",
        "acer.",
        "microsoft.",
        "logitech.",
        "jbl.",
        "bose.",
        "anker.",
        "corsair.",
        "razer.",
        "sweetwater.",
        "musiciansfriend.",
        "thomann.",
        "zappos.",
        "famousfootwear.",
        "footlocker.",
        "dsw.",
        "crocstore.",
        "allbirds.",
        "sunglasshut.",
        "warbyparker.",
        "revolve.",
        "boohoo.",
        "romwe.",
        "zaful.",
        "forever21.",
        "hollisterco.",
        "abercrombie.",
        "sephora.",
        "douglas.",
        "marionnaud.",
        "profumeriesabbioni.",
        "profumeriaweb.",
        "pinalli.",
        "limonishop.",
        "tigotastore.",
        "echarme.",
        "mybeauty.",
        "beautybay.",
        "lookfantastic.",
        "feelunique.",
        "fragrancenet.",
        "perfumesclub.",
        "parfumdreams.",
        "escentual.",
        "allbeauty.",
        "theperfumeshop.",
        "parfimo.",
        "profumerialanza.",
        "profumeriegardenia.",
        "beautyforyou.",
        "pureprofumi.",
        "profumix.",
        "profumeriapecchioli.",
        "profumeriafresia.",
        "profumeriapiazza.",
        "profumeriamarino.",
        "fragonard.",
        "profumeriadante.",
        "mondadoristore.",
        "lafeltrinelli.",
        "hoepli.",
        "ilibridi.",
        "ibs.",
        "libraccio.",
        "bibliotecaitaliana.",
        "bookrepublic.",
        "libroco.",
        "rizzolilibri.",
        "fantasybookshop.",
        "bookcitymilano.",
        "einaudi.",
        "sellerio.",
        "pandorastore.",
        "libreriarizzoli.",
        "feltrinellieditore.",
        "simonandschuster.",
        "harpercollins.",
        "penguinrandomhouse.",
        "hachettebookgroup.",
        "mcmillan.",
        "dovershop.",
        "librerialedesma.",
        "mondialibro.",
        "shop.librimondadori.",
        "letturebookshop.",
        "libriperpassione.",
        "librerialussana.",
        "leggeremania.",
        "novellalibri.",
        "storytel.",
        "toysrus.",
        "babydreams.",
        "toyscenter.",
        "bimbostore.",
        "la-citta-del-sole.",
        "legoshop.",
        "trudi.",
        "clementoni.",
        "ravensburger.",
        "hasbro.",
        "playmobil.",
        "shopdisney.",
        "animalplanetstore.",
        "barbie.",
        "playtime.",
        "magictoystore.",
        "kidzworldtoys.",
        "toybox.",
        "fisher-price.",
        "melissaanddoug.",
        "smartgames.",
        "boardgameshop.",
        "toyplanet.",
        "kidstoystore.",
        "jumbo.",
        "toys4you.",
        "toysland.",
        "giochipreziosi.",
        "mattel.",
        "megamindtoys.",
        "modellistore.",
        "woodentoys.",
        "toytown.",
        "juventus.",
        "acmilan.",
        "inter.",
        "napolicalcio.",
        "atalanta.",
        "sslazio.",
        "romacalcio.",
        "hellasverona.",
        "ucfiorentina.",
        "ussalernitana.",
        "sampdoria.",
        "leccecalcio.",
        "bolognafc.",
        "empolifc.",
        "sassuolocalcio.",
        "parmacalcio1913.",
        "reggianacalcio.",
        "spalferrara.",
        "beneventocalcio.",
        "palermocalcio.",
        "bari1908.",
        "cagliaricalcio.",
        "pisacalcio.",
        "veneziafc.",
        "monzafc.",
        "cesenacalcio.",
        "taranto.",
        "trapani.",
        "pordenonecalcio.",
        "cataniacalcio.",
        "decathlon.",
        "intersport.",
        "sportler.",
        "tuttosport.",
        "sportexpert.",
        "deporvillage.",
        "sportland.",
        "sportsdirect.",
        "dicksportinggoods.",
        "footlocker.",
        "eastbay.",
        "reebokoutlet.",
        "nikeoutlet.",
        "underarmour.",
        "fanatics.",
        "soccer.",
        "snowinn.",
        "bikeinn.",
        "runnerinn.",
        "surfdome.",
        "sportscheck.",
        "prodirectsoccer.",
        "prodirectrunning.",
        "kitbag.",
        "all4cycling.",
        "sportteam.",
        "calcioshop.",
        "weplay.",
        "evoride.",
        "tenniswarehouse.",
        "golfgalaxy.",
        "baseballmonkey.",
        "hockeymonkey.",
        "volleyballusa.",
        "climbershop.",
        "campmor.",
        "backcountry.",
        "moosejaw.",
        "booking.",
        "expedia.",
        "skyscanner.",
        "kayak.",
        "lastminute.",
        "agoda.",
        "viator.",
        "civitatis.",
        "getyourguide.",
        "travelocity.",
        "orbitz.",
        "trip.",
        "trivago.",
        "hotels.",
        "priceline.",
        "flightcentre.",
        "travel2be.",
        "bestday.",
        "etihadtravel.",
        "emiratesholidays.",
        "virginholidays.",
        "thomascook.",
        "tui.",
        "americantours.",
        "europeantravel.",
        "italianjourneys.",
        "dreamtrips.",
        "evolvi.",
        "goway.",
        "contiki.",
        "intrepidtravel.",
        "overseasadventuretravel.",
        "discoverytours.",
        "exoticjourneys.",
        "adventureworld.",
        "safaribookings.",
        "worldexpeditions.",
        "insightvacations.",
        "twitter.",
        "tiktok.",
        "pinterest.",
        "snapchat.",
        "reddit.",
        "tumblr.",
        "clubhouse.",
        "quora.",
        "medium.",
        "telegram.",
        "wechat.",
        "line.",
        "kakao.",
        "zalo.",
        "vkontakte.",
        "myspace.",
        "mastodon.",
        "beacons.",
        "onlyfans.",
        "patreon.",
        "pixiv.",
        "deviantart.",
        "youtube.",
        "vimeo.",
        "dailymotion.",
        "rumble.",
        "peertube.",
        "twitch.",
        "kick.",
        "steamcommunity.",
        "itch.",
        "roblox.",
        "discord.",
        "hertz.",
        "avis.",
        "europcar.",
        "enterprise.",
        "budget.",
        "thrifty.",
        "alamo.",
        "nationalcar.",
        "sicilybycar.",
        "goldcar.",
        "rentalcars.",
        "sixtrentacar.",
        "zipcar.",
        "dollar.",
        "autonoleggio.",
        "carrental.",
        "locautorent.",
        "maggiore.",
        "drivalia.",
        "kayak.",
        "turo.",
        "carnext.",
        "leasys.",
        "focusrent.",
        "sixt.",
        "amigoautos.",
        "fireflycarrental.",
        "carflexi.",
        "rentacaritalia.",
        "olocars.",
        "solrentacar.",
        "unicredit.",
        "intesasanpaolo.",
        "ubi.",
        "mediobanca.",
        "bper.",
        "credit-agricole.",
        "montepaschi.",
        "popso.",
        "bancobpm.",
        "carige.",
        "bnl.",
        "finecobank.",
        "chebanca.",
        "postepay.",
        "revolut.",
        "n26.",
        "hsbc.",
        "santander.",
        "barclays.",
        "deutsche-bank.",
        "raiffeisen.",
        "credit-suisse.",
        "bbva.",
        "commerzbank.",
        "abnamro.",
        "ing.",
        "dbs.",
        "citibank.",
        "bankofamerica.",
        "jpmorganchase.",
        "wellsfargo.",
        "goldmansachs.",
        "morganstanley.",
        "tdbank.",
        "bmo.",
        "rbc.",
        "cibc.",
        "bankofchina.",
        "icbc.",
        "generali.",
        "allianz.",
        "axa.",
        "zurich.",
        "realegroup.",
        "unipolsai.",
        "groupama.",
        "helvetia.",
        "mapfre.",
        "aviva.",
        "amfam.",
        "prudential.",
        "aig.",
        "metlife.",
        "geico.",
        "nationwide.",
        "statefarm.",
        "liberty-mutual.",
        "progressive.",
        "pingan.",
        "china-life.",
        "chubb.",
        "hiscox.",
        "eulerhermes.",
        "berkshirehathaway.",
        "assicurazioni.",
        "postevita.",
        "fondiaria-sai.",
        "alleanza.",
        "eurovita.",
        "italianaassicurazioni.",
        "tuaassicurazioni.",
        "vittoriassicurazioni.",
        "tempocasaassicurazioni.",
        "unicreditassicurazioni.",
        "alitalia.",
        "lufthansa.",
        "ryanair.",
        "easyjet.",
        "airfrance.",
        "britishairways.",
        "emirates.",
        "qatarairways.",
        "etihad.",
        "americanairlines.",
        "delta.",
        "united.",
        "koreanair.",
        "singaporeair.",
        "thaiairways.",
        "japanairlines.",
        "airchina.",
        "cathaypacific.",
        "aeroflot.",
        "klm.",
        "sas.",
        "finnair.",
        "tapair.",
        "austrian.",
        "iberia.",
        "swiss.",
        "virginatlantic.",
        "norwegian.",
        "bambooairways.",
        "vietnamairlines.",
        "turkishairlines.",
        "pegasusairlines.",
        "gulfair.",
        "omanair.",
        "ethiopianairlines.",
        "airmauritius.",
        "southafricanairways.",
        "airnewzealand.",
        "latam.",
        "aeromexico.",
        "copaair.",
        "avianca.",
        "wizzair.",
        "vueling.",
        "flyniki.",
        "airasia.",
        "cebuair.",
        "tigerair.",
        "jetstar.",
        "twitter.",
        "tiktok.",
        "snapchat.",
        "youtube.",
        "spotify.",
        "netflix.",
        "amazon.",
        "ebay.",
        "aliexpress.",
        "uber.",
        "lyft.",
        "google.",
        "gmail.",
        "zoom.",
        "slack.",
        "pinterest.",
        "reddit.",
        "medium.",
        "quora.",
        "clubhouse.",
        "discord.",
        "twitch.",
        "vimeo.",
        "zomato.",
        "getyourguide.",
        "booking.",
        "expedia.",
        "skyscanner.",
        "kayak.",
        "dropbox.",
        "drive.google.",
        "onedrive.",
        "github.",
        "gitlab.",
        "notion.",
        "trello.",
        "asana.",
        "monzo.",
        "n26.",
        "revolut.",
        "paypal.",
        "venmo.",
        "stripe.",
        "square.",
        "coinbase.",
        "binance.",
        "robinhood.",
        "inder.",
        "bumble.",
        "netflixparty.",
        "strava.",
        "foursquare.",
        "mapmyrun.",
        "samsung.",
        "apple.",
        "hp.",
        "lenovo.",
        "asus.",
        "dell.",
        "acer.",
        "microsoft.",
        "logitech.",
        "jbl.",
        "bose.",
        "anker.",
        "corsair.",
        "razer.",
        "sweetwater.",
        "musiciansfriend.",
        "thomann.",
        "shazam.",
        "alarmy.",
        "pandora.",
        "feedly.",
        "flipboard.",
        "bbc.",
        "cnn.",
        "nyt.",
        "theguardian.",

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
        ".fr",
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