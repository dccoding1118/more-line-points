package htmlparser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/model"
)

// HTTPFetcher abstracts HTML fetching for testing purposes.
type HTTPFetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// DefaultFetcher is the production implementation of HTTPFetcher.
type DefaultFetcher struct {
	client *http.Client
	config config.APIConfig
}

func NewDefaultFetcher(cfg config.APIConfig) *DefaultFetcher {
	return &DefaultFetcher{
		client: &http.Client{Timeout: 10 * time.Second},
		config: cfg,
	}
}

func (f *DefaultFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Origin", f.config.Headers.Origin)
	req.Header.Set("Referer", f.config.Headers.Referer)
	req.Header.Set("User-Agent", f.config.Headers.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 200 {
			b = b[:200]
		}
		return nil, fmt.Errorf("http error: %d, body: %s", resp.StatusCode, string(b))
	}

	return io.ReadAll(resp.Body)
}

// ParseResult holds the outcome of HTML analysis.
type ParseResult struct {
	Type       string
	ActionURL  string
	DailyTasks []model.DailyTask
	TaskHTML   string
}

// Parser identifies activity types and extracts tasks from HTML.
type Parser struct {
	fetcher     HTTPFetcher
	rules       *config.ParseRules
	dateExtract *DateExtractor
}

func NewParser(f HTTPFetcher, r *config.ParseRules) *Parser {
	return &Parser{
		fetcher:     f,
		rules:       r,
		dateExtract: NewDateExtractor(r.DatePatterns),
	}
}

// Parse fetches and analyzes the activity page.
func (p *Parser) Parse(ctx context.Context, act *model.Activity) (*ParseResult, error) {
	body, err := p.fetcher.Fetch(ctx, act.PageURL)
	if err != nil {
		log.Printf("[%s] [L3] fetch failed for url=%s: %v", act.ID, act.PageURL, err)
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		log.Printf("[%s] [L3] goquery parse failed: %v", act.ID, err)
		return nil, fmt.Errorf("failed to parse html: %w", err)
	}

	pageText := doc.Text()
	res := &ParseResult{Type: model.ActivityTypeOther}

	// 1. Identify Type and ActionURL
	var matchedRule *config.TypeRule
	for _, rule := range p.rules.Rules {
		if rule.URLOnly {
			// Handled by syncer pre-filtering, skip HTML parsing for these rules
			continue
		}

		// Check URL pattern presence
		firstLink := ""
		doc.Find("a").EachWithBreak(func(i int, s *goquery.Selection) bool {
			href, ok := s.Attr("href")
			if ok && strings.Contains(href, rule.URLPattern) {
				firstLink = href
				return false
			}
			return true
		})

		if firstLink == "" {
			continue
		}

		// Check text patterns if needed
		matchText := true
		if !rule.URLOnly && len(rule.TextPatterns) > 0 {
			matchText = false
			for _, pattern := range rule.TextPatterns {
				if matchPattern(pageText, pattern) {
					matchText = true
					break
				}
			}
		}

		if matchText {
			res.Type = rule.Type
			log.Printf("[%s] [L3] matched rule type: %s", act.ID, rule.Type)
			if !rule.HasDailyTasks {
				res.ActionURL = firstLink
			}
			matchedRule = &rule
			break
		}
	}

	// 2. Extract Daily Tasks if needed
	if matchedRule != nil && matchedRule.HasDailyTasks {
		log.Printf("[%s] [L3] start extracting daily tasks for type: %s", act.ID, matchedRule.Type)
		p.extractDailyTasks(doc, act, matchedRule, res)
	} else if matchedRule == nil {
		log.Printf("[%s] [L3] no rule matched, type remains: %s", act.ID, res.Type)
	}

	return res, nil
}

func (p *Parser) extractDailyTasks(doc *goquery.Document, act *model.Activity, rule *config.TypeRule, res *ParseResult) {
	var taskHTMLs []string

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || !strings.Contains(href, rule.URLPattern) {
			return
		}

		if rule.HasKeyword && strings.TrimSpace(s.Text()) == "" {
			log.Printf("[%s] [L3] skipped task with empty text for url=%s", act.ID, href)
			return
		}

		parent := findContextElement(s)
		outer, _ := goquery.OuterHtml(parent)
		taskHTMLs = append(taskHTMLs, outer)

		task := model.DailyTask{
			ActivityID: act.ID,
			URL:        href,
		}

		// Set a unique marker to pinpoint EXACTLY this node in the parent HTML
		markerID := fmt.Sprintf("target-marker-\\%d", i)
		s.SetAttr("id", markerID)

		parentHtml, _ := goquery.OuterHtml(parent)

		// Slice the parent HTML up to the node marker
		idx := strings.Index(parentHtml, markerID)
		var txt string
		if idx >= 0 {
			preHtml := parentHtml[:idx]
			txt = safeHTMLText(preHtml)
		} else {
			txt = safeHTMLText(parentHtml) // fallback
		}

		// Try to extract date from the text PRECEDING the link, taking the LAST matched date
		date, ok := p.dateExtract.ExtractLast(txt, act.ValidFrom)

		// Fallback to the original complete text if none found before the link
		if !ok {
			date, ok = p.dateExtract.Extract(safeHTMLText(parentHtml), act.ValidFrom)
		}

		if ok {
			task.UseDate = date
		}

		// Type-specific extraction
		if rule.HasKeyword {
			kw, decoded := decodeOAKeyword(href)
			if decoded {
				task.Keyword = kw
				// fallback date from keyword prefix (e.g. 0305...)
				if !ok {
					if d, isDate := p.dateExtract.ExtractFromKeyword(kw, act.ValidFrom); isDate {
						task.UseDate = d
						ok = true
					}
				}
			}
		}

		if ok {
			log.Printf("[%s] [L3] extracted task use_date=%s url=%s", act.ID, task.UseDate.Format("2006-01-02"), task.URL)
			res.DailyTasks = append(res.DailyTasks, task)
		} else {
			log.Printf("[%s] [L3] failed to extract use_date for task url=%s", act.ID, task.URL)
		}
	})

	if len(taskHTMLs) > 0 {
		res.TaskHTML = strings.Join(taskHTMLs, "")
	}
}

// findContextElement finds the nearest block-level parent (li, tr, p, div)
func findContextElement(s *goquery.Selection) *goquery.Selection {
	p := s.Parent()
	for p.Length() > 0 {
		tag := goquery.NodeName(p)
		switch tag {
		case "li", "tr", "p", "div":
			return p
		}
		p = p.Parent()
	}
	return s // fallback to itself
}

// matchPattern supports wildcard '*'
func matchPattern(text, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return strings.Contains(text, pattern)
	}

	parts := strings.Split(pattern, "*")
	lastIdx := 0
	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(text[lastIdx:], part)
		if idx == -1 {
			return false
		}
		lastIdx += idx + len(part)
	}
	return true
}

// safeHTMLText parses an HTML fragment into text, ensuring block elements and breaks
// are replaced with spaces so texts from different lines don't get concatenated.
func safeHTMLText(htmlStr string) string {
	s := strings.ReplaceAll(htmlStr, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<p>", " ")
	s = strings.ReplaceAll(s, "</p>", " ")
	s = strings.ReplaceAll(s, "<div>", " ")
	s = strings.ReplaceAll(s, "</div>", " ")
	s = strings.ReplaceAll(s, "</li>", " ")
	docTmp, _ := goquery.NewDocumentFromReader(strings.NewReader(s))
	return docTmp.Text()
}
