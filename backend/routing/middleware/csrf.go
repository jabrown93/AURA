package middleware

import (
	"aura/config"
	routes_auth "aura/routing/auth"
	"net/http"
	"net/url"
	"strings"
)

// CSRFProtect rejects state-changing requests that authenticate with the session cookie
// unless they originate from this app's own origin.
//
// Only cookie-authenticated requests need this: a browser attaches cookies to cross-site
// requests on its own, but never attaches an Authorization header. Bearer clients are
// therefore passed through untouched.
//
// SameSite=Lax on the session cookie already blocks cross-site form posts; this is the
// second line of defence, and it also covers browsers that do not honour SameSite.
func CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.Current.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if routes_auth.TokenFromSessionCookie(r) == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !requestIsSameOrigin(r) {
			sendNotAuthenticatedResponse(w, "Cross-origin request rejected")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// requestIsSameOrigin compares the browser-supplied Origin (falling back to Referer)
// against the host the request was addressed to. Browsers always send Origin for methods
// other than GET and HEAD, so a request with neither header did not come from a browser
// and has no business carrying a session cookie.
func requestIsSameOrigin(r *http.Request) bool {
	stated := r.Header.Get("Origin")
	if stated == "" {
		stated = r.Header.Get("Referer")
	}
	if stated == "" || stated == "null" {
		return false
	}

	parsed, err := url.Parse(stated)
	if err != nil || parsed.Hostname() == "" {
		return false
	}

	// Ports are ignored: cookies are not isolated by port, so matching on one would reject
	// legitimate reverse-proxy setups without adding any security.
	origin := strings.ToLower(parsed.Hostname())
	for _, host := range requestHostnames(r) {
		if host != "" && host == origin {
			return true
		}
	}
	return false
}

func requestHostnames(r *http.Request) []string {
	hosts := []string{hostnameOf(r.Host)}
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		hosts = append(hosts, hostnameOf(strings.TrimSpace(first)))
	}
	return hosts
}

// hostnameOf strips any port from a Host header value and lowercases the result.
func hostnameOf(host string) string {
	if host == "" {
		return ""
	}
	parsed, err := url.Parse("//" + host)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
