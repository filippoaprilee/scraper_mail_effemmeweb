package gmaps

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

type PlaceJob struct {
	scrapemate.Job
	UsageInResults bool
	ExtractEmail   bool
}

const (
	defaultPrio       = scrapemate.PriorityMedium
	defaultMaxRetries = 3
	defaultTimeout    = 5000 // Timeout predefinito in millisecondi
)

// NewPlaceJob crea un nuovo job per processare una URL di Google Maps
func NewPlaceJob(parentID, langCode, u string, extractEmail bool) *PlaceJob {
	return &PlaceJob{
		Job: scrapemate.Job{
			ID:         uuid.New().String(),
			ParentID:   parentID,
			Method:     "GET",
			URL:        u,
			URLParams:  map[string]string{"hl": langCode},
			MaxRetries: defaultMaxRetries,
			Priority:   defaultPrio,
		},
		UsageInResults: true,
		ExtractEmail:   extractEmail,
	}
}

// Process gestisce la risposta del job e crea un Entry o ulteriori job
func (j *PlaceJob) Process(_ context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	// Verifica che i metadati JSON siano disponibili
	raw, ok := resp.Meta["json"].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("could not extract JSON metadata from response")
	}

	// Converti i dati JSON in un Entry
	entry, err := EntryFromJSON(raw, "gmaps/gmaps_utils/cmsnames.json", "gmaps/gmaps_utils/excludewebsites.json", "gmaps/gmaps_utils/provider.json")
	if err != nil {
		return nil, nil, fmt.Errorf("error processing entry from JSON: %w", err)
	}

	// Assegna un ID e un URL all'Entry
	entry.ID = j.ParentID
	if entry.Link == "" {
		entry.Link = j.GetURL()
	}

	// Se richiesto, crea un job per estrarre email
	if j.ExtractEmail {
		emailJob := NewEmailJob(j.ID, &entry)
		j.UsageInResults = false
		return nil, []scrapemate.IJob{emailJob}, nil
	}

	return &entry, nil, nil
}

// BrowserActions gestisce le interazioni tramite Playwright per ottenere i dati
func (j *PlaceJob) BrowserActions(_ context.Context, page playwright.Page) scrapemate.Response {
	var resp scrapemate.Response

	url := j.GetURL()
	if url == "" || !strings.HasPrefix(url, "http") {
		resp.Error = fmt.Errorf("invalid URL: %s", url)
		return resp
	}

	pageResponse, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		resp.Error = fmt.Errorf("browser navigation error: %w", err)
		return resp
	}

	if err := clickRejectCookiesIfRequired(page); err != nil {
		resp.Error = fmt.Errorf("error handling cookies: %w", err)
		return resp
	}

	err = page.WaitForURL(page.URL(), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(defaultTimeout),
	})
	if err != nil {
		resp.Error = fmt.Errorf("timeout waiting for page URL: %w", err)
		return resp
	}

	rawI, err := page.Evaluate(js)
	if err != nil {
		resp.Error = fmt.Errorf("javascript evaluation error: %w", err)
		return resp
	}

	raw, ok := rawI.(string)
	if !ok {
		resp.Error = fmt.Errorf("invalid data type for evaluated JavaScript result")
		return resp
	}

	const prefix = ")]}'"
	raw = strings.TrimSpace(strings.TrimPrefix(raw, prefix))

	resp.Meta = map[string]any{"json": []byte(raw)}
	resp.URL = pageResponse.URL()
	resp.StatusCode = pageResponse.Status()

	resp.Headers = make(http.Header, len(pageResponse.Headers()))
	for k, v := range pageResponse.Headers() {
		resp.Headers.Add(k, v)
	}

	return resp
}

// UseInResults indica se questo job dovrebbe essere incluso nei risultati
func (j *PlaceJob) UseInResults() bool {
	return j.UsageInResults
}

// Script JavaScript per ottenere i dati dalla pagina
const js = `
function parse() {
  const inputString = window.APP_INITIALIZATION_STATE[3][6];
  return inputString;
}
`
