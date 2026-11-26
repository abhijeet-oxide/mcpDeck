// Package mockenv serves realistic fake versions of systems people put
// behind mcpDeck: JFrog Artifactory and Xray, Anchore Enterprise, an
// e-commerce API, GitHub, Jira, ServiceNow and Kubernetes look-alikes, an
// OAuth2 identity provider, a cookie+CSRF legacy ERP with SOAP, an
// HMAC-signed payments API, an mTLS service, GraphQL, and a flaky API with
// rate limits, async jobs and file downloads.
//
// Each system follows its real counterpart's public documentation for
// auth, paths, pagination and response shapes, so the end-to-end scenarios
// in ../scenarios exercise what users will meet. It is test tooling only:
// nothing here is shipped in the mcpdeck binary.
package mockenv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Credentials the mocks accept. Every one is a canary: scenarios assert
// none of them ever reaches an agent.
const (
	ShopUser         = "buyer@shop.example"
	ShopPass         = "shop-pass-C4N4RY-1"
	JFrogUser        = "admin"
	JFrogPass        = "jfrog-pass-C4N4RY-2"
	JFrogAPIKey      = "AKCp8jfrogapikeyC4N4RY3"
	AnchoreUser      = "admin"
	AnchorePass      = "anchore-pass-C4N4RY-4"
	OAuthClientID    = "mcpdeck-client"
	OAuthSecret      = "oauth-secret-C4N4RY-5"
	OAuthUser        = "ops@corp.example"
	OAuthUserPass    = "oauth-user-pass-C4N4RY-6"
	LegacyUser       = "clerk"
	LegacyPass       = "legacy-pass-C4N4RY-7"
	HMACKeyID        = "key_live_42"
	HMACSecret       = "hmac-secret-C4N4RY-8"
	GitHubPAT        = "ghp_githubpatC4N4RY9xxxxxxxxxxxxxxxx"
	JiraEmail        = "dev@corp.example"
	JiraToken        = "jira-api-token-C4N4RY-10"
	SNUser           = "integration"
	SNPass           = "servicenow-pass-C4N4RY-11"
	KubeToken        = "kube-sa-token-C4N4RY-12"
	GraphQLToken     = "gql-token-C4N4RY-13"
	FlakyKey         = "flaky-key-C4N4RY-14"
	WeatherKey       = "weather-key-C4N4RY-15"
	MTLSToken        = "mtls-bearer-C4N4RY-16"
	ShopRefreshToken = "never-issued-before-login"
)

// Canaries lists every secret above, for leak checks.
func Canaries() []string {
	return []string{ShopPass, JFrogPass, JFrogAPIKey, AnchorePass, OAuthSecret, OAuthUserPass, LegacyPass, HMACSecret,
		GitHubPAT, JiraToken, SNPass, KubeToken, GraphQLToken, FlakyKey, WeatherKey, MTLSToken}
}

// Env is a running set of systems.
type Env struct {
	URLs map[string]string
	// CACert trusts every HTTPS mock (PEM).
	CACert []byte
	// ClientCert and ClientKey are a client certificate the mTLS mock accepts (PEM).
	ClientCert, ClientKey []byte

	servers []*http.Server
	tokens  *tokenBook
	// TokenTTL is how long issued access tokens live. Scenarios shorten it
	// to test expiry and refresh.
	TokenTTL atomic.Int64 // nanoseconds

	expireLegacy func()
	counters     sync.Map // name → *atomic.Int64, for assertions on retries and logins
}

// Count returns how many times a named event happened (logins, retries…).
func (e *Env) Count(name string) int64 {
	v, _ := e.counters.LoadOrStore(name, new(atomic.Int64))
	return v.(*atomic.Int64).Load()
}

func (e *Env) inc(name string) int64 {
	v, _ := e.counters.LoadOrStore(name, new(atomic.Int64))
	return v.(*atomic.Int64).Add(1)
}

// Start serves every system on free 127.0.0.1 ports.
func Start() (*Env, error) {
	e := &Env{URLs: map[string]string{}, tokens: newTokenBook()}
	e.TokenTTL.Store(int64(time.Hour))
	ca, err := newCA()
	if err != nil {
		return nil, err
	}
	e.CACert = ca.certPEM
	srvCert, err := ca.issue("127.0.0.1", false)
	if err != nil {
		return nil, err
	}
	cliCert, err := ca.issue("mcpdeck-client", true)
	if err != nil {
		return nil, err
	}
	e.ClientCert, e.ClientKey = cliCert.certPEM, cliCert.keyPEM

	type sys struct {
		name  string
		h     http.Handler
		https bool
		mtls  bool
	}
	systems := []sys{
		{"shop", e.shop(), false, false},
		{"jfrog", e.jfrog(), true, false},
		{"anchore", e.anchore(), false, false},
		{"idp", e.idp(), true, false},
		{"billing", e.saasBilling(), true, false},
		{"legacy", e.legacy(), true, false},
		{"payments", e.payments(), false, false},
		{"vault", e.mtlsService(), true, true},
		{"github", e.github(), false, false},
		{"jira", e.jira(), false, false},
		{"servicenow", e.servicenow(), false, false},
		{"kube", e.kube(), true, false},
		{"graphql", e.graphqlAPI(), false, false},
		{"flaky", e.flaky(), false, false},
		{"weather", e.weather(), false, false},
	}
	for _, s := range systems {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			e.Close()
			return nil, err
		}
		scheme := "http"
		srv := &http.Server{Handler: s.h, ReadHeaderTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
		if s.https {
			scheme = "https"
			cfg := &tls.Config{Certificates: []tls.Certificate{srvCert.pair}, MinVersion: tls.VersionTLS12}
			if s.mtls {
				pool := x509.NewCertPool()
				pool.AppendCertsFromPEM(ca.certPEM)
				cfg.ClientCAs = pool
				cfg.ClientAuth = tls.RequireAndVerifyClientCert
			}
			ln = tls.NewListener(ln, cfg)
		}
		e.URLs[s.name] = fmt.Sprintf("%s://%s", scheme, ln.Addr().String())
		e.servers = append(e.servers, srv)
		go func() { _ = srv.Serve(ln) }()
	}
	return e, nil
}

// Close stops every server.
func (e *Env) Close() {
	for _, s := range e.servers {
		_ = s.Close()
	}
}

// Names lists the systems.
func (e *Env) Names() []string {
	out := make([]string, 0, len(e.URLs))
	for n := range e.URLs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ───────── tokens ─────────

type tokenBook struct {
	mu   sync.Mutex
	n    int
	live map[string]tokenInfo
}

type tokenInfo struct {
	system  string
	subject string
	expires time.Time
	refresh bool
}

func newTokenBook() *tokenBook { return &tokenBook{live: map[string]tokenInfo{}} }

func (b *tokenBook) issue(system, subject string, ttl time.Duration, refresh bool) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.n++
	kind := "at"
	if refresh {
		kind = "rt"
	}
	tok := fmt.Sprintf("%s_%s_%06d_%x", system, kind, b.n, time.Now().UnixNano()&0xffffff)
	b.live[tok] = tokenInfo{system: system, subject: subject, expires: time.Now().Add(ttl), refresh: refresh}
	return tok
}

func (b *tokenBook) valid(system, tok string, refresh bool) (tokenInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ti, ok := b.live[tok]
	if !ok || ti.system != system || ti.refresh != refresh || time.Now().After(ti.expires) {
		return ti, false
	}
	return ti, true
}

func (b *tokenBook) revoke(tok string) {
	b.mu.Lock()
	delete(b.live, tok)
	b.mu.Unlock()
}

// IssuedTokens returns every access or refresh token issued so far, for leak checks.
func (e *Env) IssuedTokens() []string {
	e.tokens.mu.Lock()
	defer e.tokens.mu.Unlock()
	out := make([]string, 0, len(e.tokens.live))
	for t := range e.tokens.live {
		out = append(out, t)
	}
	return out
}

func (e *Env) ttl() time.Duration { return time.Duration(e.TokenTTL.Load()) }

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// ───────── responses ─────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

// ───────── certificates ─────────

type certKey struct {
	certPEM, keyPEM []byte
	pair            tls.Certificate
	cert            *x509.Certificate
	key             *ecdsa.PrivateKey
}

func newCA() (*certKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "mockenv test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	c, _ := x509.ParseCertificate(der)
	return &certKey{certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), cert: c, key: key}, nil
}

func (ca *certKey) issue(name string, client bool) (*certKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	if client {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		tpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		tpl.DNSNames = []string{"localhost"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}
	kb, _ := x509.MarshalECPrivateKey(key)
	out := &certKey{certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})}
	out.pair, err = tls.X509KeyPair(out.certPEM, out.keyPEM)
	return out, err
}
