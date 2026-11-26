package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// Network is how outbound requests reach APIs (NET-1): through the system
// proxy (HTTP_PROXY, HTTPS_PROXY, NO_PROXY), no proxy, or a proxy set in
// Settings. Requests to localhost and loopback addresses never use a proxy.
type Network struct {
	Mode     string // system (default) | none | manual
	HTTP     string // proxy for http:// URLs, like http://proxy.corp:3128
	HTTPS    string // proxy for https:// URLs; HTTP when empty
	NoProxy  string // comma-separated hosts or domains that go direct; "*" for all
	User     string // proxy sign-in, if the proxy asks for one
	Password string
}

// SetNetwork changes the proxy for every request from now on.
func (p *Proxy) SetNetwork(n Network) {
	p.mu.Lock()
	p.net = n
	p.mu.Unlock()
}

// proxyFor is the transport's Proxy func.
func (p *Proxy) proxyFor(req *http.Request) (*url.URL, error) {
	p.mu.Lock()
	n := p.net
	p.mu.Unlock()
	return n.ProxyFor(req.URL)
}

// ProxyFor returns the proxy for a URL, or nil to connect directly.
func (n Network) ProxyFor(u *url.URL) (*url.URL, error) {
	host := u.Hostname()
	if isLoopback(host) {
		return nil, nil
	}
	switch n.Mode {
	case "none":
		return nil, nil
	case "manual":
	default:
		return http.ProxyFromEnvironment(&http.Request{URL: u})
	}
	if bypass(host, n.NoProxy) {
		return nil, nil
	}
	raw := n.HTTP
	if u.Scheme == "https" && strings.TrimSpace(n.HTTPS) != "" {
		raw = n.HTTPS
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	pu, err := ParseProxyURL(raw)
	if err != nil {
		return nil, err
	}
	if n.User != "" {
		pu.User = url.UserPassword(n.User, n.Password)
	}
	return pu, nil
}

// ParseProxyURL reads a proxy address; a bare host:port means http://.
func ParseProxyURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, &Error{Code: CodeUpstream, Message: fmt.Sprintf("the proxy address %q isn't valid; use host:port or http://host:port", raw)}
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return nil, &Error{Code: CodeUpstream, Message: "the proxy address has to start with http://, https:// or socks5://"}
	}
	return u, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// bypass reports whether host matches a no-proxy list: "*", a host, a
// domain (example.com or .example.com, which also match subdomains), an IP or
// a CIDR range.
func bypass(host, list string) bool {
	host = strings.ToLower(host)
	ip := net.ParseIP(host)
	for _, e := range strings.Split(list, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if e == "*" {
			return true
		}
		if _, cidr, err := net.ParseCIDR(e); err == nil {
			if ip != nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if h, _, err := net.SplitHostPort(e); err == nil {
			e = h
		}
		e = strings.TrimPrefix(e, "*")
		d := strings.TrimPrefix(e, ".")
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Request timeouts (NET-2).
const (
	DefaultRequestTimeout = 30 * time.Second
	DefaultMaxTimeout     = 300 * time.Second
)

// timeoutFor is the request's own timeout, else the default in Settings,
// capped by the maximum in Settings.
func (p *Proxy) timeoutFor(ctx context.Context, r *store.Request) time.Duration {
	sec := func(key string, def time.Duration) time.Duration {
		f, err := strconv.ParseFloat(p.store.Setting(ctx, key, ""), 64)
		if err != nil || f <= 0 {
			return def
		}
		return time.Duration(f * float64(time.Second))
	}
	max := sec("requestTimeoutMaxSeconds", DefaultMaxTimeout)
	t := sec("requestTimeoutSeconds", DefaultRequestTimeout)
	if r != nil && r.TimeoutSeconds > 0 {
		t = time.Duration(r.TimeoutSeconds * float64(time.Second))
	}
	return min(t, max)
}

// sendErr explains a failed send: a timeout, a cancel, or no connection.
func sendErr(ctx context.Context, err error, name string, timeout time.Duration) *Error {
	var ne net.Error
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return &Error{Code: CodeCanceled, Message: name + " was canceled before it finished."}
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()):
		return &Error{Code: CodeTimeout, Message: fmt.Sprintf("%s timed out after %s. Raise the request's timeout on its Settings tab, or the default in Settings → Requests.", name, timeout.Round(time.Millisecond))}
	}
	return nil
}
