package gmaps

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gosom/scrapemate"
	"github.com/mcnijman/go-emailaddress"
)

type EmailExtractJob struct {
	scrapemate.Job
	Entry *Entry
}

func NewEmailJob(parentID string, entry *Entry) *EmailExtractJob {
	const (
		defaultPrio       = scrapemate.PriorityHigh
		defaultMaxRetries = 0
	)

	return &EmailExtractJob{
		Job: scrapemate.Job{
			ParentID:   parentID,
			Method:     "GET",
			URL:        entry.WebSite,
			MaxRetries: defaultMaxRetries,
			Priority:   defaultPrio,
		},
		Entry: entry,
	}
}

func (j *EmailExtractJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer clearResponse(resp)

	if resp.Error != nil {
		return j.Entry, nil, nil
	}

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return j.Entry, nil, nil
	}

	email := extractEmail(doc, resp.Body)
	j.Entry.Email = email

	return j.Entry, nil, nil
}

func (j *EmailExtractJob) ProcessOnFetchError() bool {
	return true
}

func clearResponse(resp *scrapemate.Response) {
	resp.Document = nil
	resp.Body = nil
}

func extractEmail(doc *goquery.Document, body []byte) string {
	emails := findEmailsFromDoc(doc)

	if len(emails) == 0 {
		emails = findEmailsFromBody(body)
	}

	for _, email := range emails {
		sanitized := sanitizeEmail(email)
		if isValidEmail(sanitized) {
			return sanitized
		}
	}

	return ""
}

func findEmailsFromDoc(doc *goquery.Document) []string {
	var emails []string
	doc.Find("a[href^='mailto:']").Each(func(_ int, s *goquery.Selection) {
		if mailto, exists := s.Attr("href"); exists {
			email := strings.TrimPrefix(mailto, "mailto:")
			emails = append(emails, email)
		}
	})
	return emails
}

func findEmailsFromBody(body []byte) []string {
	var emails []string
	addresses := emailaddress.Find(body, false)
	for _, addr := range addresses {
		emails = append(emails, addr.String())
	}
	return emails
}

func sanitizeEmail(email string) string {
	return strings.ReplaceAll(email, "%20", "")
}

func isValidEmail(email string) bool {
	if len(email) > 100 {
		return false
	}
	_, err := emailaddress.Parse(strings.TrimSpace(email))
	return err == nil
}

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

func findEmailsWithRegex(body []byte) []string {
	return emailRegex.FindAllString(string(body), -1)
}
