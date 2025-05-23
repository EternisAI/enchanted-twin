package types

import (
	"encoding/json"
	"regexp"
	"strings"
)

var imageURLRE = regexp.MustCompile(
	`https?://[^ \t\r\n\[\]"')>]+?\.(?:jpe?g|png|gif|webp|bmp|svg|tiff)(?:\?[^\s\[\]"')>]+)?`,
)

// ExtractImageURLs returns every image URL it can find, de-duplicated.
func extractImageURLs(s string) []string {
	seen := make(map[string]struct{})
	add := func(u string) {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
		}
	}

	// 1️⃣  Regex pass (works for both plain text and “array as string” cases)
	for _, m := range imageURLRE.FindAllString(s, -1) {
		add(m)
	}

	// 2️⃣  If the whole payload looks like a JSON array, try to unmarshal too.
	trim := strings.TrimSpace(s)
	if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
		var arr []string
		if err := json.Unmarshal([]byte(trim), &arr); err == nil {
			for _, u := range arr {
				if imageURLRE.MatchString(u) {
					add(u)
				}
			}
		}
	}

	// Collect keys → slice
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	return out
}
