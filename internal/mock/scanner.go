package mock

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

// The scanner is a container-security API for the full builder flow
// (MOCK-6): log in for a token, list applications, list an application's
// images, then list an image's vulnerabilities. It serves HTTPS with a
// self-signed certificate, so the TLS settings (API-9) are part of the
// walk-through, and its vulnerability list is deliberately large so the
// guidelines (GUIDE-3) have something to say.
const (
	ScannerUser = "deck"
	ScannerPass = "demo-scanner-pass-5e1d"
	scannerTok  = "scn_7a1f9c2e4b6d8a0f"
)

type scanApp struct {
	ID, Name, Team, Repo, Lang, Created string
}

var scanApps = []scanApp{
	{"0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a01", "checkout", "payments", "github.com/acme/checkout", "go", "2024-02-11T10:00:00Z"},
	{"0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a02", "storefront", "web", "github.com/acme/storefront", "typescript", "2023-07-03T08:30:00Z"},
	{"0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a03", "ledger", "finance", "github.com/acme/ledger", "java", "2022-11-19T16:45:00Z"},
}

type scanImage struct {
	ID, AppID, Repo, Tag, Pushed string
	Size                         int
}

var scanImages = []scanImage{
	{"img-7d41a0", "0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a01", "registry.acme.example/checkout", "1.42.0", "2026-11-20T09:12:00Z", 48211234},
	{"img-7d41a1", "0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a01", "registry.acme.example/checkout", "1.41.3", "2026-11-02T14:01:00Z", 48120022},
	{"img-91c2b0", "0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a02", "registry.acme.example/storefront", "5.8.1", "2026-11-22T18:40:00Z", 131002411},
	{"img-a03f10", "0b7e3f0a-6c1d-4f1e-9a51-1c2d3e4f5a03", "registry.acme.example/ledger", "3.0.0-rc2", "2026-10-30T07:55:00Z", 302114001},
}

var sevs = []string{"CRITICAL", "HIGH", "HIGH", "MEDIUM", "MEDIUM", "MEDIUM", "LOW", "LOW", "LOW", "NEGLIGIBLE"}
var pkgs = []string{"openssl", "libxml2", "glibc", "zlib", "curl", "busybox", "golang.org/x/net", "express", "lodash", "log4j-core", "jackson-databind", "netty", "ncurses", "sqlite"}

// vulns builds a deterministic, deliberately bloated list for one image.
func vulns(imageID string) []map[string]any {
	seed := 0
	for _, c := range imageID {
		seed = seed*31 + int(c)
	}
	n := 60 + seed%30
	out := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		k := (seed + i*7) % 1000
		sev := sevs[(seed+i)%len(sevs)]
		pkg := pkgs[(seed/3+i)%len(pkgs)]
		score := map[string]float64{"CRITICAL": 9.8, "HIGH": 7.5, "MEDIUM": 5.3, "LOW": 3.1, "NEGLIGIBLE": 0.0}[sev]
		cve := fmt.Sprintf("CVE-2026-%04d", 1000+k)
		fixed := ""
		if i%3 != 0 {
			fixed = fmt.Sprintf("1.%d.%d", k%9, k%13+1)
		}
		out = append(out, map[string]any{
			"id":                fmt.Sprintf("%s-%03d", imageID, i),
			"cve":               cve,
			"severity":          sev,
			"status":            map[bool]string{true: "fixable", false: "no_fix"}[fixed != ""],
			"package":           map[string]any{"name": pkg, "version": fmt.Sprintf("1.%d.%d", k%9, k%13), "type": "os", "purl": "pkg:generic/" + pkg, "licenses": []string{"MIT"}},
			"fixedVersion":      fixed,
			"cvss":              map[string]any{"version": "3.1", "score": score, "vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", "source": "nvd", "exploitability": 3.9, "impact": 5.9},
			"title":             fmt.Sprintf("%s: issue in %s", cve, pkg),
			"description":       strings.Repeat(fmt.Sprintf("A flaw was found in %s that may allow a remote attacker to cause a denial of service or execute code. ", pkg), 4),
			"references":        []string{"https://nvd.example/vuln/detail/" + cve, "https://security.example/advisories/" + cve, "https://github.example/" + pkg + "/issues/" + fmt.Sprint(k)},
			"publishedAt":       "2026-0" + fmt.Sprint(1+k%9) + "-1" + fmt.Sprint(k%9) + "T00:00:00Z",
			"modifiedAt":        "2026-11-0" + fmt.Sprint(1+k%9) + "T00:00:00Z",
			"layer":             map[string]any{"digest": "sha256:" + strings.Repeat(fmt.Sprintf("%02x", k%256), 32), "createdBy": "RUN apt-get install -y " + pkg, "index": k % 12},
			"epss":              map[string]any{"score": float64(k%100) / 1000, "percentile": float64(k%100) / 100},
			"kev":               sev == "CRITICAL" && k%2 == 0,
			"cwe":               []string{fmt.Sprintf("CWE-%d", 20+k%400)},
			"vendorSeverity":    map[string]any{"nvd": score, "redhat": sev, "ubuntu": strings.ToLower(sev)},
			"suppressed":        false,
			"suppressionReason": nil,
			"firstSeen":         "2026-10-0" + fmt.Sprint(1+k%9) + "T00:00:00Z",
			"scanner":           map[string]any{"name": "acme-scan", "version": "4.2.0", "db": "2026-11-24"},
			"rawFinding":        base64.StdEncoding.EncodeToString([]byte(strings.Repeat(cve+"|"+pkg+"|", 12))),
		})
	}
	return out
}

func (w *World) scanner() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+scannerTok {
				writeJSON(rw, 401, map[string]any{"error": "unauthorized", "message": "log in with POST /api/v1/auth/login and send Authorization: Bearer <access_token>"})
				return
			}
			h(rw, r)
		}
	}
	mux.HandleFunc("GET /openapi.json", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(scannerSpec("https://" + r.Host)))
	})
	mux.HandleFunc("POST /api/v1/auth/login", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Username != ScannerUser || in.Password != ScannerPass {
			writeJSON(rw, 401, map[string]any{"error": "invalid_credentials", "message": "wrong username or password"})
			return
		}
		writeJSON(rw, 200, map[string]any{"access_token": scannerTok, "token_type": "Bearer", "expires_in": 3600,
			"refresh_token": "scn_refresh_" + strings.Repeat("x", 24), "scope": "read:apps read:images read:vulns",
			"user": map[string]any{"id": "u-1001", "username": ScannerUser, "displayName": "Deck Demo", "roles": []string{"viewer"}, "tenant": "acme"}})
	})
	mux.HandleFunc("GET /api/v1/applications", auth(func(rw http.ResponseWriter, r *http.Request) {
		items := []map[string]any{}
		q := strings.ToLower(r.URL.Query().Get("name"))
		for _, a := range scanApps {
			if q != "" && !strings.Contains(a.Name, q) {
				continue
			}
			items = append(items, map[string]any{"id": a.ID, "name": a.Name, "team": a.Team, "repository": a.Repo, "language": a.Lang,
				"createdAt": a.Created, "labels": map[string]any{"tier": "prod", "owner": a.Team}, "riskScore": len(a.Name) * 7})
		}
		writeJSON(rw, 200, map[string]any{"items": items, "total": len(items), "page": 1, "pageSize": 50})
	}))
	mux.HandleFunc("GET /api/v1/applications/{appId}/images", auth(func(rw http.ResponseWriter, r *http.Request) {
		items := []map[string]any{}
		for _, im := range scanImages {
			if im.AppID == r.PathValue("appId") {
				items = append(items, map[string]any{"id": im.ID, "repository": im.Repo, "tag": im.Tag, "digest": "sha256:" + strings.Repeat(im.ID[4:6], 32),
					"sizeBytes": im.Size, "pushedAt": im.Pushed, "os": "linux", "arch": "amd64", "lastScannedAt": "2026-11-24T03:00:00Z"})
			}
		}
		if len(items) == 0 {
			writeJSON(rw, 404, map[string]any{"error": "not_found", "message": "no application with id " + r.PathValue("appId")})
			return
		}
		writeJSON(rw, 200, map[string]any{"items": items, "total": len(items)})
	}))
	mux.HandleFunc("GET /api/v1/images/{imageId}/vulnerabilities", auth(func(rw http.ResponseWriter, r *http.Request) {
		id := r.PathValue("imageId")
		found := false
		for _, im := range scanImages {
			found = found || im.ID == id
		}
		if !found {
			writeJSON(rw, 404, map[string]any{"error": "not_found", "message": "no image " + id})
			return
		}
		items := vulns(id)
		if s := strings.ToUpper(r.URL.Query().Get("severity")); s != "" {
			kept := items[:0]
			for _, v := range items {
				if v["severity"] == s {
					kept = append(kept, v)
				}
			}
			items = kept
		}
		writeJSON(rw, 200, map[string]any{"imageId": id, "items": items, "total": len(items), "scannedAt": "2026-11-24T03:00:00Z"})
	}))
	return mux
}

// selfSigned makes a certificate for 127.0.0.1 and localhost, valid a year.
func selfSigned() (tls.Certificate, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "mcpdeck demo scanner"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"}}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	kb, _ := x509.MarshalECPrivateKey(key)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	c, err := tls.X509KeyPair(pemCert, pemKey)
	return c, pemCert, err
}

func scannerSpec(base string) string {
	return `{
  "openapi": "3.0.3",
  "info": { "title": "Acme Scanner", "version": "4.2.0", "description": "Container image scanning: applications, images and vulnerabilities." },
  "servers": [{ "url": "` + base + `" }],
  "paths": {
    "/api/v1/auth/login": { "post": { "operationId": "login", "summary": "Log in and get an access token", "security": [],
      "requestBody": { "content": { "application/json": { "schema": { "type": "object", "required": ["username", "password"],
        "properties": { "username": { "type": "string", "example": "deck" }, "password": { "type": "string", "example": "" } } } } } },
      "responses": { "200": { "description": "Token" }, "401": { "description": "Wrong credentials" } } } },
    "/api/v1/applications": { "get": { "operationId": "listApplications", "summary": "List applications",
      "parameters": [{ "name": "name", "in": "query", "schema": { "type": "string" } }],
      "responses": { "200": { "description": "Applications" } } } },
    "/api/v1/applications/{appId}/images": { "get": { "operationId": "listImages", "summary": "List an application's images",
      "parameters": [{ "name": "appId", "in": "path", "required": true, "schema": { "type": "string" } }],
      "responses": { "200": { "description": "Images" }, "404": { "description": "No such application" } } } },
    "/api/v1/images/{imageId}/vulnerabilities": { "get": { "operationId": "listVulnerabilities", "summary": "List an image's vulnerabilities",
      "parameters": [{ "name": "imageId", "in": "path", "required": true, "schema": { "type": "string" } },
        { "name": "severity", "in": "query", "schema": { "type": "string", "enum": ["CRITICAL", "HIGH", "MEDIUM", "LOW"] } }],
      "responses": { "200": { "description": "Vulnerabilities" } } } }
  }
}`
}
