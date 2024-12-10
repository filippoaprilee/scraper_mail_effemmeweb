package gmaps

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "runtime/debug"
    "strings"
    "time"

    "github.com/PuerkitoBio/goquery"
)

type Address struct {
	Borough    string `json:"borough"`
	Street     string `json:"street"`
	City       string
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
	CookieBanner string `json:"cookie_banner"` // Nuovo campo per Cookie Banner
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

// detectProtocol verifica se l'URL usa HTTP o HTTPS.
func detectProtocol(url string) (string, error) {
	if strings.HasPrefix(url, "https://") {
		return "https", nil
	} else if strings.HasPrefix(url, "http://") {
		return "http", nil
	}

	// Helper per tentare connessioni
	tryProtocol := func(proto string) bool {
		_, err := http.Get(proto + url)
		return err == nil
	}

	if tryProtocol("https://") {
		return "https", nil
	}
	if tryProtocol("http://") {
		return "http", nil
	}
	return "", fmt.Errorf("protocollo non rilevato per URL: %s", url)
}

// detectTechnology analizza gli script della pagina per identificare la tecnologia.
func detectTechnology(url string) (string, error) {
	// Configura il client HTTP con timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Effettua la richiesta HTTP
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("errore durante il fetch dell'URL %s: %v", url, err)
	}
	defer resp.Body.Close()

	// Leggi il contenuto HTML
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "", fmt.Errorf("errore durante la lettura del contenuto HTML: %v", err)
	}
	html := buf.String()

	// Parsing del DOM
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("errore durante il parsing del DOM: %v", err)
	}

	// Mappa di parole chiave per tecnologie
	technologyKeywords := map[string]string{
		"wp-content":          "WordPress",
		"wp-users":            "WordPress",
		"<!-- WordPress":      "WordPress",
		"shopify":             "Shopify",
		"magento":             "Magento",
		"prestashop":          "PrestaShop",
		"drupal":              "Drupal",
		"joomla":              "Joomla",
		"ghost":               "Ghost",
		"django":              "Django",
		"flask":               "Flask",
		"laravel":             "Laravel",
		"react":               "React",
		"vue":                 "Vue.js",
		"angular":             "Angular",
		"rails":               "Ruby on Rails",
		"woocommerce":         "WooCommerce",
		"content=\"WordPress": "WordPress",
		"<!DOCTYPE html>":     "HTML Puro",
	}

	// Cerca parole chiave direttamente nell'HTML
	for keyword, tech := range technologyKeywords {
		if strings.Contains(html, keyword) {
			return tech, nil
		}
	}

	// Analizza i meta tag
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			for keyword, tech := range technologyKeywords {
				if strings.Contains(content, keyword) {
					tech = tech
				}
			}
		}
	})

	// Analizza i tag <script> e <link>
	doc.Find("script, link").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			for keyword, tech := range technologyKeywords {
				if strings.Contains(src, keyword) {
					tech = tech
				}
			}
		}
	})

	// Analizza commenti HTML
	comments := extractComment(html)
	for _, comment := range comments {
		for keyword, tech := range technologyKeywords {
			if strings.Contains(comment, keyword) {
				return tech, nil
			}
		}
	}
	// Nessuna tecnologia rilevata
	return "Altro", nil
}

// extractComment estrae i commenti HTML.
func extractComment(html string) []string {
	var comments []string
	startIdx := strings.Index(html, "<!--")
	for startIdx != -1 {
		endIdx := strings.Index(html[startIdx:], "-->")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx + len("-->")
		comments = append(comments, html[startIdx:endIdx])
		html = html[endIdx:]
		startIdx = strings.Index(html, "<!--")
	}
	return comments
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
		"Cookie Banner", // Nuova colonna
	}
}

func (e *Entry) CsvRow() []string {
	if e.Title == "" || e.Phone == "" {
		return nil
	}
	return []string{
		e.Title,
		e.Category,
		e.WebSite,
		e.Protocol,
		e.Technology,
		e.Phone,
		e.Street,
		e.City,
		e.Province,
		e.Email,
		e.CookieBanner, // Nuovo valore
	}
}

func EntryFromJSON(raw []byte) (Entry, error) {
	var entry Entry
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v\nStack: %s\n", r, debug.Stack())
		}
	}()

	var jd []any
	if err := json.Unmarshal(raw, &jd); err != nil {
		return entry, err
	}

	if len(jd) < 7 {
		return entry, fmt.Errorf("invalid json")
	}

	darray, ok := jd[6].([]any)
	if !ok {
		return entry, fmt.Errorf("invalid json structure")
	}

	entry.Title = getNthElementAndCast[string](darray, 11)
	entry.Category = getNthElementAndCast[string](darray, 13, 0)
	entry.WebSite = getNthElementAndCast[string](darray, 7, 0)
	entry.Phone = getNthElementAndCast[string](darray, 178, 0, 0)
	address := Address{
		Street:  getNthElementAndCast[string](darray, 183, 1, 1),
		City:    getNthElementAndCast[string](darray, 183, 1, 3),
		State:   getNthElementAndCast[string](darray, 183, 1, 5),
		Country: getNthElementAndCast[string](darray, 183, 1, 6),
	}
	entry.Street = address.Street
	entry.City = address.City
	entry.Province = strings.Replace(address.State, "Province of ", "", 1)

	// Esclude i siti web specificati
	if entry.WebSite != "" {
		if isExcludedWebsite(entry.WebSite) {
			return entry, fmt.Errorf("sito web escluso: %s", entry.WebSite)
		}

		if protocol, err := detectProtocol(entry.WebSite); err == nil {
			entry.Protocol = protocol
		}
		if technology, err := detectTechnology(entry.WebSite); err == nil {
			entry.Technology = technology
		}
		if cookieBanner, err := detectCookieBanner(entry.WebSite); err == nil {
			entry.CookieBanner = cookieBanner
		}
	}

	return entry, nil
}

// isExcludedWebsite verifica se il sito web deve essere escluso
func isExcludedWebsite(url string) bool {
	excludedDomains := []string{
		"miodottore.com",
		"instagram.com",
		"facebook.com",
		"booking.com",
		"airbnb.com",
		"twitter.com",
		"linkedin.com",
		"tiktok.com",
		"youtube.com",
		"pinterest.com",
		"snapchat.com",
		"amazon.com",
		"ebay.com",
		"tripadvisor.com",
		"expedia.com",
		"hotels.com",
		"trivago.com",
		"skyscanner.com",
		"subito.it",
	}

	for _, domain := range excludedDomains {
		if strings.Contains(strings.ToLower(url), domain) {
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

func getNthElementAndCast[T any](arr []any, indexes ...int) T {
	var defaultVal T
	for len(indexes) > 1 {
		idx := indexes[0]
		indexes = indexes[1:]
		if idx >= len(arr) {
			return defaultVal
		}
		next, ok := arr[idx].([]any)
		if !ok {
			return defaultVal
		}
		arr = next
	}
	if len(indexes) == 0 || indexes[0] >= len(arr) {
		return defaultVal
	}
	value, ok := arr[indexes[0]].(T)
	if !ok {
		return defaultVal
	}
	return value
}