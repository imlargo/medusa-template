package tools

import (
	"fmt"
	"net/url"
	"strings"
)

// CleanHostURL removes the protocol prefix (http:// or https://) from a URL.
// This is useful when you need just the host part for comparison or storage.
//
// Example:
//
//	clean := tools.CleanHostURL("https://example.com") // Returns: "example.com"
//	clean := tools.CleanHostURL("http://localhost:8080") // Returns: "localhost:8080"
func CleanHostURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	rawURL = strings.TrimPrefix(rawURL, "http://")
	rawURL = strings.TrimPrefix(rawURL, "https://")
	return rawURL
}

// ToQueryParams converts a map of parameters into an encoded query string.
// Empty values are skipped. Keys and values are URL-encoded, and the output is
// sorted by key so the same map always produces the same string.
//
// Example:
//
//	tools.ToQueryParams(map[string]string{"name": "John Doe", "q": "a&b", "skip": ""})
//	// Returns: "name=John+Doe&q=a%26b"
func ToQueryParams(params map[string]string) string {
	values := make(url.Values, len(params))
	for key, value := range params {
		if value != "" {
			values.Set(key, value)
		}
	}

	// url.Values.Encode sorts by key.
	return values.Encode()
}

// IsLocalhost checks if a URL or host string points to a localhost address.
// Supports various localhost representations:
//   - "localhost"
//   - "127.0.0.1" and other 127.x.x.x addresses
//   - "::1" (IPv6 localhost)
//   - "0.0.0.0"
//   - "[::1]" (IPv6 with brackets)
//
// Works with or without protocol, and handles ports correctly.
//
// Example:
//
//	tools.IsLocalhost("http://localhost:8080")  // Returns: true
//	tools.IsLocalhost("localhost:8080")         // Returns: true
//	tools.IsLocalhost("127.0.0.1")              // Returns: true
//	tools.IsLocalhost("127.5.10.1")             // Returns: true
//	tools.IsLocalhost("::1")                    // Returns: true
//	tools.IsLocalhost("example.com")            // Returns: false
func IsLocalhost(urlOrHost string) bool {

	u, err := url.Parse(urlOrHost)
	var host string

	if err == nil && u.Host != "" {
		host = u.Hostname()
	} else {

		host = urlOrHost

		if idx := strings.LastIndex(host, ":"); idx != -1 {
			possiblePort := host[idx+1:]

			if len(possiblePort) > 0 && possiblePort[0] >= '0' && possiblePort[0] <= '9' {
				host = host[:idx]
			}
		}
	}

	host = strings.ToLower(strings.TrimSpace(host))

	localhosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"[::1]",
	}

	for _, lh := range localhosts {
		if host == lh {
			return true
		}
	}

	if strings.HasPrefix(host, "127.") {
		return true
	}

	return false
}

// IsHTTPS checks if a URL uses the HTTPS protocol.
// Only works with full URLs (containing "://").
// Returns false for hosts without a protocol.
//
// Example:
//
//	tools.IsHTTPS("https://example.com") // Returns: true
//	tools.IsHTTPS("http://example.com")  // Returns: false
//	tools.IsHTTPS("example.com")         // Returns: false (no protocol)
func IsHTTPS(urlOrHost string) bool {

	if !strings.Contains(urlOrHost, "://") {

		return false
	}

	u, err := url.Parse(urlOrHost)
	if err != nil {
		return false
	}

	return strings.ToLower(u.Scheme) == "https"
}

// ToCompleteURL converts a host or partial URL to a complete URL with protocol.
// Automatically determines the protocol:
//   - Uses "http://" for localhost addresses
//   - Uses "https://" for all other hosts
//
// If the input already contains a valid protocol, it returns the input unchanged.
//
// Example:
//
//	tools.ToCompleteURL("example.com")           // Returns: "https://example.com"
//	tools.ToCompleteURL("localhost:8080")        // Returns: "http://localhost:8080"
//	tools.ToCompleteURL("https://example.com")   // Returns: "https://example.com" (unchanged)
func ToCompleteURL(urlOrHost string) string {
	urlOrHost = strings.TrimSpace(urlOrHost)

	if strings.Contains(urlOrHost, "://") {

		if u, err := url.Parse(urlOrHost); err == nil && u.Scheme != "" {
			return urlOrHost
		}
	}

	scheme := "https"
	if IsLocalhost(urlOrHost) {
		scheme = "http"
	}

	completeURL := fmt.Sprintf("%s://%s", scheme, urlOrHost)

	if _, err := url.Parse(completeURL); err != nil {

		return urlOrHost
	}

	return completeURL
}

// GetFullAppUrl builds a complete application URL from a host and port.
// Handles localhost detection and protocol selection automatically.
//
// Example:
//
//	tools.GetFullAppUrl("localhost", 8080)     // Returns: "http://localhost:8080"
//	tools.GetFullAppUrl("example.com", 443)    // Returns: "https://example.com"
//	tools.GetFullAppUrl("api.example.com", 80) // Returns: "https://api.example.com"
func GetFullAppUrl(host string, port int) string {
	host = CleanHostURL(host)

	if IsLocalhost(host) {
		return fmt.Sprintf("http://%s:%d", host, port)
	}

	return ToCompleteURL(host)
}

// GetFullDocsUrl builds a complete URL to the API documentation page.
// Assumes documentation is served at "/docs/index.html".
//
// Example:
//
//	tools.GetFullDocsUrl("localhost", 8080)  // Returns: "http://localhost:8080/docs/index.html"
//	tools.GetFullDocsUrl("api.example.com", 443) // Returns: "https://api.example.com/docs/index.html"
func GetFullDocsUrl(host string, port int) string {
	return GetFullAppUrl(host, port) + "/docs/index.html"
}
