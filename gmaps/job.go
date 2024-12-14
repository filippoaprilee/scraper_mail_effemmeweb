package gmaps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

func init() {
	// Crea un logger silenzioso
	silentLogger := logrus.New()
	silentLogger.SetOutput(io.Discard)

	// Sovrascrivi il logger standard di logrus
	logrus.SetOutput(io.Discard)
	logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: false,
	})

	// Disabilita completamente stderr
	disableStdErr()
}

func disableStdErr() {
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
}

type GmapJob struct {
	scrapemate.Job
	LangCode     string
	MaxDepth     int
	ExtractEmail bool
}

func NewGmapJob(id, langCode, query string, maxDepth int, extractEmail bool) *GmapJob {
	if id == "" {
		id = uuid.New().String()
	}

	return &GmapJob{
		Job: scrapemate.Job{
			ID:         id,
			Method:     http.MethodGet,
			URL:        "https://www.google.com/maps/search/" + url.QueryEscape(query),
			URLParams:  map[string]string{"hl": langCode},
			MaxRetries: 3,
			Priority:   scrapemate.PriorityLow,
		},
		LangCode:     langCode,
		MaxDepth:     maxDepth,
		ExtractEmail: extractEmail,
	}
}

func (j *GmapJob) UseInResults() bool {
	return false
}

func (j *GmapJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer cleanResponse(resp)

	if resp.Error != nil {
		return nil, nil, resp.Error
	}

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse document")
	}

	return nil, j.extractJobsFromDocument(doc, resp.URL), nil
}

func (j *GmapJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
	pageResponse, err := page.Goto(j.GetFullURL(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return scrapemate.Response{Error: fmt.Errorf("navigation failed: %w", err)}
	}

	if err = clickRejectCookiesIfRequired(page); err != nil {
		return scrapemate.Response{Error: fmt.Errorf("cookie rejection failed: %w", err)}
	}

	if _, err = scroll(ctx, page, j.MaxDepth); err != nil {
		return scrapemate.Response{Error: fmt.Errorf("scrolling failed: %w", err)}
	}

	body, err := page.Content()
	if err != nil {
		return scrapemate.Response{Error: fmt.Errorf("content retrieval failed: %w", err)}
	}

	return scrapemate.Response{
		URL:        pageResponse.URL(),
		StatusCode: pageResponse.Status(),
		Body:       []byte(body),
	}
}

func (j *GmapJob) extractJobsFromDocument(doc *goquery.Document, baseURL string) []scrapemate.IJob {
	var nextJobs []scrapemate.IJob

	if strings.Contains(baseURL, "/maps/place/") {
		nextJobs = append(nextJobs, NewPlaceJob(j.ID, j.LangCode, baseURL, j.ExtractEmail))
	} else {
		doc.Find(`div[role=feed] div[jsaction]>a`).Each(func(_ int, s *goquery.Selection) {
			if href := s.AttrOr("href", ""); href != "" {
				nextJobs = append(nextJobs, NewPlaceJob(j.ID, j.LangCode, href, j.ExtractEmail))
			}
		})
	}
	return nextJobs
}

func clickRejectCookiesIfRequired(page playwright.Page) error {
	selector := `form[action="https://consent.google.com/save"]:first-of-type button:first-of-type`
	const timeout = 500

	el, err := page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(timeout),
	})
	if err != nil || el == nil {
		return nil
	}
	return el.Click()
}

func scroll(ctx context.Context, page playwright.Page, maxDepth int) (int, error) {
	const scrollSelector = `div[role='feed']`
	const scrollScript = `async () => {
		const el = document.querySelector("` + scrollSelector + `");
		if (!el) return null;
		el.scrollTop = el.scrollHeight;
		return el.scrollHeight;
	}`

	var currentScrollHeight int
	waitTime := 100.0
	const maxWait = 2000

	for i := 0; i < maxDepth; i++ {
		scrollHeight, err := page.Evaluate(scrollScript)
		if err != nil || scrollHeight == nil {
			return i, fmt.Errorf("scroll evaluation failed")
		}

		height, ok := scrollHeight.(int)
		if !ok || height == currentScrollHeight {
			break
		}

		currentScrollHeight = height
		page.WaitForTimeout(waitTime)
		waitTime = minFloat(waitTime*1.5, maxWait)
	}
	return maxDepth, nil
}

func cleanResponse(resp *scrapemate.Response) {
	resp.Document = nil
	resp.Body = nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}