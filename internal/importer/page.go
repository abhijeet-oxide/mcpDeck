package importer

import (
	"net/url"
	"regexp"
	"strings"
)

var specURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)spec-url\s*=\s*["']([^"']+)["']`),                                     // Redoc
	regexp.MustCompile(`(?i)Redoc\.init\(\s*["']([^"']+)["']`),                                    // Redoc JS
	regexp.MustCompile(`(?i)SwaggerUIBundle\(\s*\{[^}]*?\burl\s*:\s*["']([^"']+)["']`),            // Swagger UI
	regexp.MustCompile(`(?i)\burls\s*:\s*\[\s*\{[^}]*?\burl\s*:\s*["']([^"']+)["']`),              // Swagger UI, many specs
	regexp.MustCompile(`(?i)["']((?:[^"']*/)?(?:openapi|swagger)(?:\.v\d)?\.(?:json|ya?ml))["']`), // plain link
}

// IsHTML reports whether a fetched document is a web page.
func IsHTML(contentType string, body []byte) bool {
	if strings.Contains(contentType, "html") {
		return true
	}
	t := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 200)])))
	return strings.HasPrefix(t, "<!doctype html") || strings.HasPrefix(t, "<html")
}

// FindSpecURL finds the spec a Swagger UI or Redoc page loads (IMP-2).
// Swagger UI often keeps its config in swagger-initializer.js; the second
// return value is that script's URL when the page itself has no spec URL.
func FindSpecURL(pageURL string, html []byte) (spec string, script string) {
	base, _ := url.Parse(pageURL)
	for _, re := range specURLPatterns {
		if m := re.FindSubmatch(html); m != nil {
			if u, err := base.Parse(string(m[1])); err == nil {
				return u.String(), ""
			}
		}
	}
	if m := regexp.MustCompile(`src=["']([^"']*swagger-initializer\.js)["']`).FindSubmatch(html); m != nil {
		if u, err := base.Parse(string(m[1])); err == nil {
			return "", u.String()
		}
	}
	return "", ""
}

// NoSpecFound is the IMP-2 message.
const NoSpecFound = "no spec found on that page; paste the spec URL instead"
