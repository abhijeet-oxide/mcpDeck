package deck

import (
	"context"
	"strconv"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
)

// Network and timeout settings (NET-1, NET-2). The proxy password is a
// secret: it is stored sealed and never returned, only whether it is set.
const proxyPasswordKey = "proxyPasswordSealed"

// ApplyNetwork hands the stored proxy settings to the proxy.
func (d *Deck) ApplyNetwork(ctx context.Context) {
	if d.Proxy == nil {
		return
	}
	n := proxy.Network{
		Mode:    d.Store.Setting(ctx, "proxyMode", "system"),
		HTTP:    d.Store.Setting(ctx, "proxyHttp", ""),
		HTTPS:   d.Store.Setting(ctx, "proxyHttps", ""),
		NoProxy: d.Store.Setting(ctx, "noProxy", ""),
		User:    d.Store.Setting(ctx, "proxyUser", ""),
	}
	if sealed := d.Store.Setting(ctx, proxyPasswordKey, ""); sealed != "" && d.Vault != nil {
		if pw, err := d.Vault.Unseal(sealed); err == nil {
			n.Password = pw
		}
	}
	d.Proxy.SetNetwork(n)
}

func checkNetSetting(k, v string) error {
	switch k {
	case "proxyMode":
		if v != "system" && v != "none" && v != "manual" {
			return badRequest("proxyMode is system, none or manual")
		}
	case "proxyHttp", "proxyHttps":
		if v != "" {
			if _, err := proxy.ParseProxyURL(v); err != nil {
				return badRequest("%s", err.Error())
			}
		}
	case "requestTimeoutSeconds", "requestTimeoutMaxSeconds":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 3600 {
			return badRequest("%s must be between 1 and 3600 seconds", k)
		}
	}
	return nil
}
