package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// apiCmd lists and shows imported APIs and sets their TLS (PAR-1, API-9,
// API-10), so a terminal or agent sees what the UI's application pages show.
func apiCmd(data *string) *cobra.Command {
	c := &cobra.Command{Use: "api", Short: "List and inspect APIs, and set their TLS (CA, client certificate)"}

	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List APIs with their request counts", Example: "  mcpdeck api list --json", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			var apis []map[string]any
			if err := cl.do("GET", "/api/apis", nil, &apis); err != nil {
				return err
			}
			rows := []map[string]any{}
			for _, a := range apis {
				authType := ""
				if au, ok := a["auth"].(map[string]any); ok {
					authType = fmt.Sprint(au["type"])
				}
				rows = append(rows, map[string]any{"name": a["name"], "title": a["title"], "requests": a["requests"], "auth": authType,
					"tools": len(asSlice(a["tools"])), "needsSecret": a["needsSecret"]})
			}
			if listJSON {
				printJSON(rows)
				return nil
			}
			if len(rows) == 0 {
				fmt.Println("No APIs yet. Next: mcpdeck import <url-or-file> --api <name>")
			}
			for _, r := range rows {
				fmt.Printf("%-20v %4v requests  auth: %v\n", r["name"], r["requests"], r["auth"])
			}
			return nil
		}}
	list.Flags().BoolVar(&listJSON, "json", false, "print JSON")

	var showJSON bool
	show := &cobra.Command{Use: "show <api>", Short: "One API: auth type, variables (secret values never shown) and requests with access",
		Example: "  mcpdeck api show shop --json", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			d, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			var raw map[string]any
			if err := cl.do("GET", "/api/apis/"+d.ID, nil, &raw); err != nil {
				return err
			}
			reqs := []map[string]any{}
			for _, rr := range d.Requests {
				var m map[string]any
				_ = json.Unmarshal(rr, &m)
				reqs = append(reqs, map[string]any{"name": m["name"], "method": m["method"], "path": strings.TrimPrefix(fmt.Sprint(m["url"]), "{{baseUrl}}"), "access": m["access"]})
			}
			vars := []map[string]any{}
			for _, v := range d.Variables {
				vars = append(vars, map[string]any{"name": v.Name, "source": v.Source})
			}
			authType := ""
			if a, ok := raw["auth"].(map[string]any); ok {
				authType = fmt.Sprint(a["type"])
			}
			out := map[string]any{"name": d.Name, "auth": authType, "variables": vars, "requests": reqs}
			if showJSON {
				printJSON(out)
				return nil
			}
			fmt.Printf("%s (auth: %s)\n\nVariables:\n", d.Name, authType)
			for _, v := range vars {
				fmt.Printf("  %v (%v)\n", v["name"], v["source"])
			}
			fmt.Println("\nRequests:")
			for _, r := range reqs {
				fmt.Printf("  %-28v %-6v %v  %v\n", r["name"], r["method"], r["path"], r["access"])
			}
			return nil
		}}
	show.Flags().BoolVar(&showJSON, "json", false, "print JSON")

	var caFile, certFile, keyVar string
	var insecure, verify bool
	tls := &cobra.Command{Use: "tls <api>", Short: "Trust a CA, send a client certificate (mutual TLS), or skip verification",
		Example: "  mcpdeck api tls vault --ca-file corp-ca.pem --client-cert client.pem --client-key-var clientKey\n" +
			"  cat client.key | mcpdeck secret set vault clientKey   # the key itself is a secret\n" +
			"  mcpdeck api tls lab --insecure",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := localClient(*data)
			if err != nil {
				return err
			}
			d, err := findAPI(cl, args[0])
			if err != nil {
				return err
			}
			var raw struct {
				TLS map[string]any `json:"tls"`
			}
			if err := cl.do("GET", "/api/apis/"+d.ID, nil, &raw); err != nil {
				return err
			}
			t := raw.TLS
			if t == nil {
				t = map[string]any{}
			}
			read := func(f string) (string, error) {
				b, err := os.ReadFile(f)
				return string(b), err
			}
			if caFile != "" {
				if t["caCerts"], err = read(caFile); err != nil {
					return err
				}
			}
			if certFile != "" {
				if t["clientCert"], err = read(certFile); err != nil {
					return err
				}
			}
			if keyVar != "" {
				t["clientKey"] = "{{" + strings.Trim(keyVar, "{} ") + "}}"
			}
			if insecure {
				t["insecure"] = true
			}
			if verify {
				t["insecure"] = false
			}
			if err := cl.do("PATCH", "/api/apis/"+d.ID, map[string]any{"tls": t}, nil); err != nil {
				return err
			}
			fmt.Printf("Updated TLS for %s.\n", args[0])
			return nil
		}}
	tls.Flags().StringVar(&caFile, "ca-file", "", "PEM file of CA certificates to trust, on top of the system ones")
	tls.Flags().StringVar(&certFile, "client-cert", "", "PEM file of the client certificate for mutual TLS")
	tls.Flags().StringVar(&keyVar, "client-key-var", "", "secret variable holding the client key (set it with mcpdeck secret set)")
	tls.Flags().BoolVar(&insecure, "insecure", false, "skip certificate verification (self-signed lab servers)")
	tls.Flags().BoolVar(&verify, "verify", false, "turn certificate verification back on")

	c.AddCommand(list, show, tls)
	return c
}
