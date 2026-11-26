package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
)

// OAuth2, cookie sessions with CSRF, HMAC signing, mTLS and API keys.

func init() {
	// ── OAuth2 ──
	add("onboarding", "billing spec's oauth2 scheme becomes OAuth2 auth with its token URL", func(t *testing.T) {
		h.importURL(t, "billingSpec", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billingSpec")
		expect(t, a.Auth["type"] == "oauth2" && a.Auth["tokenUrl"] == h.env.URLs["idp"]+"/oauth2/token", "imported auth: %s", asJSON(a.Auth))
	})

	add("auth", "OAuth2 client credentials token fetched and used", func(t *testing.T) {
		need(t, "billing")
		before := h.env.Count("idp.client_credentials")
		a, name := one(t, "read", []string{"billing"}, "Count open invoices.", "", nil,
			`  const r = await billing.listInvoices({ status: "open", limit: 100 });
  return r.data.length;`)
		out := a.ok(name, map[string]any{})
		expect(t, num(out) == 23, "got %v", out)
		expect(t, h.env.Count("idp.client_credentials") > before, "no token was fetched from the IdP")
		h.noLeaksInCallLog(t)
	})

	add("auth", "OAuth2 token cached until it expires", func(t *testing.T) {
		need(t, "billing")
		a, name := one(t, "read", []string{"billing"}, "One invoice id.", "", nil, `  return (await billing.listInvoices({ limit: 1 })).data[0].id;`)
		a.ok(name, map[string]any{})
		before := h.env.Count("idp.client_credentials")
		for i := 0; i < 4; i++ {
			a.ok(name, map[string]any{})
		}
		expect(t, h.env.Count("idp.client_credentials") == before, "fetched %d new tokens for 4 calls", h.env.Count("idp.client_credentials")-before)
	})

	add("auth", "OAuth2 token renewed after expires_in", func(t *testing.T) {
		need(t, "billing")
		h.env.TokenTTL.Store(int64(1200 * time.Millisecond))
		defer h.env.TokenTTL.Store(int64(time.Hour))
		a, name := one(t, "read", []string{"billing"}, "One invoice id.", "", nil, `  return (await billing.listInvoices({ limit: 1 })).data[0].id;`)
		a.ok(name, map[string]any{})
		time.Sleep(1500 * time.Millisecond)
		a.ok(name, map[string]any{})
	})

	add("auth", "OAuth2 token URL that is not http or https is refused", func(t *testing.T) {
		h.importURL(t, "billingFile", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billingFile")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "client_credentials", "tokenUrl": "file:///etc/passwd",
			"clientId": mockenv.OAuthClientID, "clientSecret": "x"})
		ag, name := one(t, "read", []string{"billingFile"}, "One customer.", "", nil, `  return await billingFile.getCustomer({ id: "cus_003" });`)
		ag.fails(name, map[string]any{}, "absolute http or https URL")
	})

	add("auth", "OAuth2 client secret sent in the body instead of basic", func(t *testing.T) {
		h.importURL(t, "billingBody", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billingBody")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, a.ID, "clientSecret", mockenv.OAuthSecret, true)
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "client_credentials", "tokenUrl": h.env.URLs["idp"] + "/oauth2/token",
			"clientId": mockenv.OAuthClientID, "clientSecret": "{{clientSecret}}", "clientAuth": "body"})
		ag, name := one(t, "read", []string{"billingBody"}, "One customer.", "", nil, `  return await billingBody.getCustomer({ id: "cus_003" });`)
		out := ag.ok(name, map[string]any{})
		expect(t, get(out, "delinquent") == true, "got %s", asJSON(out))
	})

	add("auth", "OAuth2 password grant with refresh token rotation", func(t *testing.T) {
		h.importURL(t, "billingUser", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billingUser")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, a.ID, "clientSecret", mockenv.OAuthSecret, true)
		h.setVar(t, a.ID, "password", mockenv.OAuthUserPass, true)
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "password", "tokenUrl": h.env.URLs["idp"] + "/oauth2/token",
			"clientId": mockenv.OAuthClientID, "clientSecret": "{{clientSecret}}", "username": mockenv.OAuthUser, "password": "{{password}}"})
		h.env.TokenTTL.Store(int64(1200 * time.Millisecond))
		defer h.env.TokenTTL.Store(int64(time.Hour))
		ag, name := one(t, "read", []string{"billingUser"}, "One invoice id.", "", nil, `  return (await billingUser.listInvoices({ limit: 1 })).data[0].id;`)
		pw, rf := h.env.Count("idp.password"), h.env.Count("idp.refresh")
		ag.ok(name, map[string]any{})
		time.Sleep(1500 * time.Millisecond)
		ag.ok(name, map[string]any{})
		expect(t, h.env.Count("idp.refresh") > rf, "the refresh token was never used")
		expect(t, h.env.Count("idp.password")-pw == 1, "sent the password %d times", h.env.Count("idp.password")-pw)
	})

	add("errors", "OAuth2 wrong client secret gives a clear auth error", func(t *testing.T) {
		h.importURL(t, "billingBad", h.env.URLs["billing"]+"/openapi.json")
		a := h.apiByName(t, "billingBad")
		h.patchAPI(t, a.ID, map[string]any{"tls": map[string]any{"caCerts": string(h.env.CACert)}})
		h.setVar(t, a.ID, "clientSecret", "wrong-secret", true)
		h.setAuth(t, a.ID, map[string]any{"type": "oauth2", "grant": "client_credentials", "tokenUrl": h.env.URLs["idp"] + "/oauth2/token",
			"clientId": mockenv.OAuthClientID, "clientSecret": "{{clientSecret}}"})
		ag, name := one(t, "read", []string{"billingBad"}, "One invoice id.", "", nil, `  return (await billingBad.listInvoices({ limit: 1 })).data[0].id;`)
		ag.fails(name, map[string]any{}, "invalid_client")
	})

	add("pagination", "billing cursor pagination with starting_after and has_more", func(t *testing.T) {
		need(t, "billing")
		a, name := one(t, "read", []string{"billing"}, "Every invoice id.", "", nil,
			`  const ids: string[] = [];
  let after: string | undefined;
  for (;;) {
    const r = await billing.listInvoices(after ? { limit: 20, starting_after: after } : { limit: 20 });
    ids.push(...r.data.map((x: any) => x.id));
    if (!r.has_more) break;
    after = r.data[r.data.length - 1].id;
  }
  return { count: ids.length, unique: new Set(ids).size };`)
		out := a.ok(name, map[string]any{})
		expect(t, num(get(out, "count")) == 57 && num(get(out, "unique")) == 57, "got %s", asJSON(out))
	})

	add("chaining", "billing invoices joined with customers", func(t *testing.T) {
		need(t, "billing")
		a, name := one(t, "read", []string{"billing"}, "Uncollectible invoices with the customer's email.", "", nil,
			`  const r = await billing.listInvoices({ status: "uncollectible", limit: 100 });
  const ids = [...new Set(r.data.map((x: any) => x.customer))] as string[];
  const cs = await Promise.all(ids.map((id) => billing.getCustomer({ id })));
  const email = Object.fromEntries(cs.map((c: any) => [c.id, c.email]));
  return r.data.map((x: any) => ({ id: x.id, due: x.amount_due, email: email[x.customer] }));`)
		out := a.ok(name, map[string]any{}).([]any)
		expect(t, len(out) == 11 && strings.Contains(asJSON(out[0]), "@corp.example"), "got %s", asJSON(out))
	})

	// ── Cookie session with CSRF ──
	add("auth", "legacy CSRF login page, form post, session cookie", func(t *testing.T) {
		need(t, "legacy")
		a, name := one(t, "read", []string{"legacy"}, "Search customers.", `q: { type: "string" }`, []string{"q"},
			`  const r = await call("legacy", "GET /api/customers/", { q: input.q });
  return r.results.map((c: any) => c.name);`)
		out := a.ok(name, map[string]any{"q": "north"})
		expect(t, asJSON(out) == `["Northwind Traders"]`, "got %s", asJSON(out))
		h.noLeaksInCallLog(t)
	})

	add("auth", "legacy CSRF header on writes comes from the login", func(t *testing.T) {
		need(t, "legacy")
		k := kit(t, "legacy")
		name := uniq("note")
		h.saveTool(t, k, toolSrc(name, "Add a note to a customer.", `id: { type: "string" }, text: { type: "string" }`, []string{"id", "text"},
			`  return await call("legacy", "POST /api/customers/" + input.id + "/notes/", { body: { text: input.text } });`))
		before := h.env.Count("legacy.note")
		a := h.agentFor(t, k, "read_write")
		out := a.ok(name, map[string]any{"id": "C100", "text": "Called about invoice"})
		expect(t, get(out, "customer") == "C100" && h.env.Count("legacy.note") == before+1, "got %s", asJSON(out))
	})

	add("auth", "legacy expired session redirect triggers a fresh login", func(t *testing.T) {
		need(t, "legacy")
		a, name := one(t, "read", []string{"legacy"}, "Count customers.", "", nil, `  return (await call("legacy", "GET /api/customers/", {})).count;`)
		a.ok(name, map[string]any{})
		h.env.ExpireSessions()
		before := h.env.Count("legacy.login")
		out := a.ok(name, map[string]any{})
		expect(t, num(out) == 3, "got %v", out)
		expect(t, h.env.Count("legacy.login") == before+1, "did not log in again after the session ended")
	})

	add("payloads", "legacy SOAP call parsed with xml()", func(t *testing.T) {
		need(t, "legacy")
		a, name := one(t, "read", []string{"legacy"}, "Credit limit from the ERP.", `id: { type: "string" }`, []string{"id"},
			"  const env = `<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\"><soap:Body><GetCreditLimit xmlns=\"urn:erp\"><CustomerId>${input.id}</CustomerId></GetCreditLimit></soap:Body></soap:Envelope>`;\n"+
				`  const text = await call("legacy", "POST /soap/CreditService", { body: env, headers: { "Content-Type": "text/xml; charset=utf-8", SOAPAction: "urn:GetCreditLimit" } });
  const doc = xml(text);
  const r = doc.Envelope.Body.GetCreditLimitResponse;
  return { id: r.CustomerId, limit: Number(r.CreditLimit["#text"]), currency: r.CreditLimit["@currency"], status: r.Status, holds: [].concat(r.Holds.Hold).map((x: any) => x.Code) };`)
		out := a.ok(name, map[string]any{"id": "C200"})
		expect(t, num(get(out, "limit")) == 120000 && get(out, "currency") == "USD" && asJSON(get(out, "holds")) == `["LATE"]`, "got %s", asJSON(out))
	})

	add("errors", "legacy SOAP fault surfaces its faultstring", func(t *testing.T) {
		need(t, "legacy")
		a, name := one(t, "read", []string{"legacy"}, "Credit limit from the ERP.", `id: { type: "string" }`, []string{"id"},
			"  const env = `<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\"><soap:Body><GetCreditLimit xmlns=\"urn:erp\"><CustomerId>${input.id}</CustomerId></GetCreditLimit></soap:Body></soap:Envelope>`;\n"+
				`  return await call("legacy", "POST /soap/CreditService", { body: env, headers: { "Content-Type": "text/xml", SOAPAction: "urn:GetCreditLimit" } });`)
		a.fails(name, map[string]any{"id": "C999"}, "500", "Unknown customer C999")
	})

	// ── HMAC ──
	add("auth", "payments HMAC-signed GET", func(t *testing.T) {
		need(t, "payments")
		before := h.env.Count("payments.ok")
		a, name := one(t, "read", []string{"payments"}, "Recent payments.", "", nil,
			`  const r = await payments.listPayments({ limit: 4 });
  return r.payments.map((p: any) => p.id + ":" + p.status);`)
		out := a.ok(name, map[string]any{})
		expect(t, len(out.([]any)) == 4 && h.env.Count("payments.ok") > before, "got %s", asJSON(out))
	})

	add("auth", "payments HMAC signs the body of a POST", func(t *testing.T) {
		need(t, "payments")
		k := kit(t, "payments")
		name := uniq("refund")
		h.saveTool(t, k, toolSrc(name, "Refund a payment once.", `payment: { type: "string" }, key: { type: "string" }`, []string{"payment", "key"},
			`  return await payments.createRefund({ body: { payment: input.payment, amount: 500 }, headers: { "Idempotency-Key": input.key } });`))
		a := h.agentFor(t, k, "read_write")
		bad := h.env.Count("payments.bad_signature")
		r1 := a.ok(name, map[string]any{"payment": "pay_0001", "key": name})
		r2 := a.ok(name, map[string]any{"payment": "pay_0001", "key": name})
		expect(t, h.env.Count("payments.bad_signature") == bad, "a signature was rejected")
		expect(t, get(r1, "id") == get(r2, "id"), "the idempotency key was not honoured: %s vs %s", asJSON(r1), asJSON(r2))
	})

	add("auth", "payments signs path parameters and query strings exactly", func(t *testing.T) {
		need(t, "payments")
		a, name := one(t, "read", []string{"payments"}, "One payment.", `id: { type: "string" }`, []string{"id"},
			`  return await payments.getPayment({ id: input.id });`)
		out := a.ok(name, map[string]any{"id": "pay 0001/x"})
		expect(t, get(out, "status") == "captured", "got %s", asJSON(out))
	})

	// ── mTLS ──
	add("auth", "vault mTLS client certificate plus token", func(t *testing.T) {
		need(t, "vault")
		a, name := one(t, "read", []string{"vault"}, "Vault health and the client name it saw.", "", nil,
			`  return await call("vault", "GET /v1/sys/health", {});`)
		out := a.ok(name, map[string]any{})
		expect(t, get(out, "sealed") == false && get(out, "client_cn") != "", "got %s", asJSON(out))
	})

	add("secrets", "vault client key never shown back", func(t *testing.T) {
		a := need(t, "vault")
		var raw map[string]any
		h.mustAPI(t, "GET", "/api/apis/"+a.ID, nil, &raw)
		expect(t, !strings.Contains(asJSON(raw), "PRIVATE KEY"), "the private key is visible in the API settings")
		ag, name := one(t, "read", []string{"vault"}, "Secret metadata.", "", nil, `  return await call("vault", "GET /v1/secret/metadata/apps/payments", {});`)
		ag.ok(name, map[string]any{})
	})

	// ── API keys ──
	add("auth", "API key in the query string", func(t *testing.T) {
		need(t, "weather")
		a, name := one(t, "read", []string{"weather"}, "Current weather.", `city: { type: "string" }`, []string{"city"},
			`  const r = await weather.currentWeather({ q: input.city, units: "metric" });
  return { city: r.name, temp: r.main.temp, sky: r.weather[0].main };`)
		out := a.ok(name, map[string]any{"city": "Pune"})
		expect(t, get(out, "city") == "Pune", "got %s", asJSON(out))
		h.noLeaksInCallLog(t)
	})

	add("auth", "API key in a header", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "Slow call.", "", nil, `  return await flaky.getSlow({ ms: 10 });`)
		a.ok(name, map[string]any{})
	})

	add("secrets", "an echo endpoint can't reveal the API key", func(t *testing.T) {
		need(t, "flaky")
		a, name := one(t, "read", []string{"flaky"}, "What the server received.", "", nil, `  return await flaky.echo({});`)
		a.ok(name, map[string]any{})
		h.noLeaksInCallLog(t)
	})

	add("secrets", "a network error doesn't reveal a query-string key", func(t *testing.T) {
		h.importURL(t, "weatherDown", h.env.URLs["weather"]+"/openapi.json")
		a := h.apiByName(t, "weatherDown")
		h.setVar(t, a.ID, "apiKey", mockenv.WeatherKey, true)
		h.setVar(t, a.ID, "baseUrl", "http://127.0.0.1:1", false)
		h.patchAPI(t, a.ID, map[string]any{"allowedHosts": []string{"127.0.0.1:1"}})
		ag, name := one(t, "read", []string{"weatherDown"}, "Weather.", "", nil, `  return await weatherDown.currentWeather({ q: "x" });`)
		ag.fails(name, map[string]any{})
		h.noLeaksInCallLog(t)
	})
}
