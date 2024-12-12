package gmaps

import (
	"context"
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

	job := EmailExtractJob{
		Job: scrapemate.Job{
			ParentID:   parentID,
			Method:     "GET",
			URL:        entry.WebSite,
			MaxRetries: defaultMaxRetries,
			Priority:   defaultPrio,
		},
	}

	job.Entry = entry

	return &job
}

func (j *EmailExtractJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
	}()

	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("Processing email job", "url", j.URL)

	// if html fetch failed just return
	if resp.Error != nil {
		return j.Entry, nil, nil
	}

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return j.Entry, nil, nil
	}

	// Extract and validate email
	var email string
	emails := docEmailExtractor(doc)
	if len(emails) == 0 {
		emails = regexEmailExtractor(resp.Body)
	}

	if len(emails) > 0 {
		// Sanitize the email
		sanitizedEmail := sanitizeEmail(emails[0])
		if isValidEmail(sanitizedEmail) {
			email = sanitizedEmail
		}
	}

	j.Entry.Email = email

	return j.Entry, nil, nil
}

func (j *EmailExtractJob) ProcessOnFetchError() bool {
	return true
}

func docEmailExtractor(doc *goquery.Document) []string {
	seen := map[string]bool{}
	var emails []string

	doc.Find("a[href^='mailto:']").Each(func(_ int, s *goquery.Selection) {
		mailto, exists := s.Attr("href")
		if exists {
			value := strings.TrimPrefix(mailto, "mailto:")
			value = sanitizeEmail(value) // Sanitize the email
			if isValidEmail(value) && !seen[value] {
				emails = append(emails, value)
				seen[value] = true
			}
		}
	})

	return emails
}

func regexEmailExtractor(body []byte) []string {
	seen := map[string]bool{}
	var emails []string

	addresses := emailaddress.Find(body, false)
	for _, address := range addresses {
		email := address.String()
		email = sanitizeEmail(email) // Sanitize the email
		if isValidEmail(email) && !seen[email] {
			emails = append(emails, email)
			seen[email] = true
		}
	}

	return emails
}

func sanitizeEmail(email string) string {
	// Remove unwanted characters like "%20"
	return strings.ReplaceAll(email, "%20", "")
}

func isValidEmail(email string) bool {
	if len(email) > 100 {
		return false
	}
	// Check if the email is valid
	_, err := emailaddress.Parse(strings.TrimSpace(email))
	return err == nil
}
