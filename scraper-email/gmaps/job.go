package gmaps

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
	"github.com/playwright-community/playwright-go"
)

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

	job := GmapJob{
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

	return &job
}

func (j *GmapJob) UseInResults() bool {
	return false
}

func (j *GmapJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
	}()

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("could not convert to goquery document")
	}

	var nextJobs []scrapemate.IJob

	if strings.Contains(resp.URL, "/maps/place/") {
		nextJobs = append(nextJobs, NewPlaceJob(j.ID, j.LangCode, resp.URL, j.ExtractEmail))
	} else {
		doc.Find(`div[role=feed] div[jsaction]>a`).Each(func(_ int, s *goquery.Selection) {
			if href := s.AttrOr("href", ""); href != "" {
				nextJobs = append(nextJobs, NewPlaceJob(j.ID, j.LangCode, href, j.ExtractEmail))
			}
		})
	}

	return nil, nextJobs, nil
}

func (j *GmapJob) BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response {
	var resp scrapemate.Response

	pageResponse, err := page.Goto(j.GetFullURL(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		resp.Error = err
		return resp
	}

	if err = clickRejectCookiesIfRequired(page); err != nil {
		resp.Error = err
		return resp
	}

	const defaultTimeout = 5000
	err = page.WaitForURL(page.URL(), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(defaultTimeout),
	})
	if err != nil {
		resp.Error = err
		return resp
	}

	resp.URL = pageResponse.URL()
	resp.StatusCode = pageResponse.Status()
	resp.Headers = make(http.Header, len(pageResponse.Headers()))
	for k, v := range pageResponse.Headers() {
		resp.Headers.Add(k, v)
	}

	_, err = scroll(ctx, page, j.MaxDepth)
	if err != nil {
		resp.Error = err
		return resp
	}

	body, err := page.Content()
	if err != nil {
		resp.Error = err
		return resp
	}

	resp.Body = []byte(body)
	return resp
}

// clickRejectCookiesIfRequired handles cookie consent dialogs
func clickRejectCookiesIfRequired(page playwright.Page) error {
	sel := `form[action="https://consent.google.com/save"]:first-of-type button:first-of-type`
	const timeout = 500
	el, err := page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(timeout),
	})
	if err != nil || el == nil {
		return nil
	}
	return el.Click()
}

// scroll handles scrolling on the page
func scroll(ctx context.Context, page playwright.Page, maxDepth int) (int, error) {
	scrollSelector := `div[role='feed']`
	expr := `async () => {
		const el = document.querySelector("` + scrollSelector + `");
		el.scrollTop = el.scrollHeight;
		return new Promise((resolve) => {
			setTimeout(() => resolve(el.scrollHeight), %d);
		});
	}`

	var currentScrollHeight int
	waitTime := 100.0
	cnt := 0
	const maxWait = 2000

	for i := 0; i < maxDepth; i++ {
		cnt++
		scrollHeight, err := page.Evaluate(fmt.Sprintf(expr, maxWait))
		if err != nil {
			return cnt, err
		}

		height, ok := scrollHeight.(int)
		if !ok || height == currentScrollHeight {
			break
		}

		currentScrollHeight = height
		select {
		case <-ctx.Done():
			return currentScrollHeight, nil
		default:
			page.WaitForTimeout(waitTime)
		}

		waitTime = minFloat(waitTime*1.5, maxWait)
	}

	return cnt, nil
}

// minFloat returns the minimum of two float64 values
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
