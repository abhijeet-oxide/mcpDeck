package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Guard keeps the local server local (RUN-5).
type Guard struct {
	Port     int
	CLIToken string

	mu       sync.Mutex
	launch   map[string]time.Time // one-time launch tokens
	sessions map[string]bool      // UI cookie values
}

const cookieName = "mcpdeck_session"

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewGuard returns a guard with a fresh CLI token.
func NewGuard(port int) *Guard {
	return &Guard{Port: port, CLIToken: randHex(24), launch: map[string]time.Time{}, sessions: map[string]bool{}}
}

// LaunchURL makes a one-time link that sets the UI cookie (RUN-2).
func (g *Guard) LaunchURL(path string) string {
	t := randHex(16)
	g.mu.Lock()
	g.launch[t] = time.Now().Add(10 * time.Minute)
	g.mu.Unlock()
	if path == "" {
		path = "/"
	}
	return "http://127.0.0.1:" + strconv.Itoa(g.Port) + "/launch?t=" + t + "&next=" + url.QueryEscape(path)
}

// HostOK rejects DNS rebinding: Host must be 127.0.0.1, localhost or [::1] on our port.
func (g *Guard) HostOK(r *http.Request) bool {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || port != strconv.Itoa(g.Port) {
		return false
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// OriginOK accepts no Origin or our own.
func (g *Guard) OriginOK(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		// Browsers send Sec-Fetch-Site on cross-site requests even without Origin.
		s := r.Header.Get("Sec-Fetch-Site")
		return s == "" || s == "same-origin" || s == "none"
	}
	u, err := url.Parse(o)
	if err != nil || u.Scheme != "http" || u.Port() != strconv.Itoa(g.Port) {
		return false
	}
	h := u.Hostname()
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

// Authed reports whether an /api request carries the UI cookie or the CLI token.
// Agent tokens are never accepted here (ACC-6).
func (g *Guard) Authed(r *http.Request) (who string, ok bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(g.CLIToken)) == 1 {
			return "cli", true
		}
		return "", false
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return "ui", g.sessions[c.Value]
}

// Launch exchanges a one-time token for the UI cookie.
func (g *Guard) Launch(w http.ResponseWriter, r *http.Request) {
	t := r.URL.Query().Get("t")
	g.mu.Lock()
	exp, ok := g.launch[t]
	delete(g.launch, t)
	var sid string
	if ok && time.Now().Before(exp) {
		sid = randHex(24)
		g.sessions[sid] = true
	}
	g.mu.Unlock()
	if sid == "" {
		http.Error(w, "This launch link was already used or has expired. Run mcpdeck again to open a new one.", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	next := r.URL.Query().Get("next")
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusFound)
}
