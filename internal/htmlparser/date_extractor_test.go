package htmlparser

import (
	"testing"
	"time"
)

func TestDateExtractor_Extract(t *testing.T) {
	extractor := NewDateExtractor([]string{`(\d{2})/(\d{2})`})
	validFrom := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

	// Test valid
	date, ok := extractor.Extract("abc 01/05 def", validFrom)
	if !ok || date.Year() != 2026 || date.Month() != 1 || date.Day() != 5 {
		t.Errorf("Extract failed: %v, %v", ok, date)
	}

	// Test less than 3 matches
	extractorBad := NewDateExtractor([]string{`(\d{2})/`})
	_, ok = extractorBad.Extract("abc 01/ def", validFrom)
	if ok {
		t.Errorf("Extract should fail")
	}

	// Test no match
	_, ok = extractor.Extract("abc def", validFrom)
	if ok {
		t.Errorf("Extract should fail on no match")
	}
}

func TestDateExtractor_ExtractFromKeyword(t *testing.T) {
	extractor := NewDateExtractor(nil)
	validFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Test valid
	date, ok := extractor.ExtractFromKeyword("0305KEY", validFrom)
	if !ok || date.Month() != 3 || date.Day() != 5 {
		t.Errorf("ExtractFromKeyword failed: %v, %v", ok, date)
	}

	// Test short
	_, ok = extractor.ExtractFromKeyword("123", validFrom)
	if ok {
		t.Errorf("Expected fail on short keyword")
	}

	// Test no match
	_, ok = extractor.ExtractFromKeyword("ABCDKEY", validFrom)
	if ok {
		t.Errorf("Expected fail on no match")
	}
}
