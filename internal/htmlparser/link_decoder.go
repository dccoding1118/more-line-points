package htmlparser

import (
	"net/url"
	"strings"
)

// decodeOAKeyword extracts the @channel_id and keyword from a LINE OA message URL.
// Example: https://line.me/R/oaMessage/@linegiftshoptw/?0304美食扭蛋機
func decodeOAKeyword(link string) (string, bool) {
	u, err := url.Parse(link)
	if err != nil {
		return "", false
	}

	// 1. Path contains /R/oaMessage/@channel_id/
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "R" || pathParts[1] != "oaMessage" {
		return "", false
	}

	// 2. Query contains the keyword (e.g., ?0304美食扭蛋機)
	kw := u.RawQuery // RawQuery contains what comes after '?'
	if kw == "" {
		return "", false
	}

	decoded, err := url.QueryUnescape(kw)
	if err != nil {
		return kw, true // fallback to raw string
	}

	return decoded, true
}
