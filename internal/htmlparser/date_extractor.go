package htmlparser

import (
	"regexp"
	"strconv"
	"time"
)

// DateExtractor extracts specific dates from strings based on patterns.
type DateExtractor struct {
	patterns []*regexp.Regexp
}

func NewDateExtractor(regexStrings []string) *DateExtractor {
	var compile []*regexp.Regexp
	for _, s := range regexStrings {
		re, err := regexp.Compile(s)
		if err == nil {
			compile = append(compile, re)
		}
	}
	return &DateExtractor{patterns: compile}
}

// Extract picks a date (Month/Day) from the text, inferring the year from ValidFrom.
func (e *DateExtractor) Extract(text string, validFrom time.Time) (time.Time, bool) {
	for _, re := range e.patterns {
		matches := re.FindStringSubmatch(text)
		if len(matches) < 3 {
			continue
		}

		month, _ := strconv.Atoi(matches[1])
		day, _ := strconv.Atoi(matches[2])

		if isValidDateValues(month, day) {
			return e.constructDate(month, day, validFrom), true
		}
	}
	return time.Time{}, false
}

func isValidDateValues(month, day int) bool {
	return month >= 1 && month <= 12 && day >= 1 && day <= 31
}

// ExtractLast picks the *last* matched date (Month/Day) from the text, inferring the year from ValidFrom.
func (e *DateExtractor) ExtractLast(text string, validFrom time.Time) (time.Time, bool) {
	bestIndex := -1
	var bestTime time.Time
	var found bool

	for _, re := range e.patterns {
		matches := re.FindAllStringSubmatchIndex(text, -1)
		for _, m := range matches {
			if len(m) < 6 {
				continue
			}
			matchIdx := m[0]
			if matchIdx > bestIndex {
				month, _ := strconv.Atoi(text[m[2]:m[3]])
				day, _ := strconv.Atoi(text[m[4]:m[5]])
				if isValidDateValues(month, day) {
					bestIndex = matchIdx
					bestTime = e.constructDate(month, day, validFrom)
					found = true
				}
			}
		}
	}

	return bestTime, found
}

// ExtractFromKeyword specifically handles MMDD prefix (e.g., 0305...)
func (e *DateExtractor) ExtractFromKeyword(kw string, validFrom time.Time) (time.Time, bool) {
	if len(kw) < 4 {
		return time.Time{}, false
	}
	re := regexp.MustCompile(`^(\d{2})(\d{2})`)
	matches := re.FindStringSubmatch(kw)
	if len(matches) < 3 {
		return time.Time{}, false
	}

	month, _ := strconv.Atoi(matches[1])
	day, _ := strconv.Atoi(matches[2])

	return e.constructDate(month, day, validFrom), true
}

func (e *DateExtractor) constructDate(month, day int, validFrom time.Time) time.Time {
	year := validFrom.Year()

	// Wrap year if parsed month precedes validFrom month (crossing new year)
	if month < int(validFrom.Month()) {
		year++
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
