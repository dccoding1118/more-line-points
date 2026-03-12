package htmlparser

import "testing"

func TestDecodeOAKeyword(t *testing.T) {
	// success
	kw, ok := decodeOAKeyword("https://line.me/R/oaMessage/@id/?K1")
	if !ok || kw != "K1" {
		t.Errorf("failed decoded value: %q %v", kw, ok)
	}

	// fail: not oaMessage
	_, ok = decodeOAKeyword("https://line.me/other")
	if ok {
		t.Errorf("expected fail")
	}

	// fail: no query
	_, ok = decodeOAKeyword("https://line.me/R/oaMessage/@id")
	if ok {
		t.Errorf("expected fail due to missing query")
	}
}
