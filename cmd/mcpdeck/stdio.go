package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newRawGet(c *client, path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", c.base+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

// stdioCmd bridges stdio-only clients to a toolkit's endpoint (TK-2).
func stdioCmd() *cobra.Command {
	var toolkit, base string
	var build bool
	c := &cobra.Command{Use: "stdio --toolkit <name>", Short: "Bridge a stdio MCP client to a toolkit; reads the token from MCPDECK_TOKEN",
		Example: "  MCPDECK_TOKEN=mcpd_… mcpdeck stdio --toolkit support",
		RunE: func(*cobra.Command, []string) error {
			tok := os.Getenv("MCPDECK_TOKEN")
			if tok == "" {
				return errors.New("set MCPDECK_TOKEN to the agent token from mcpDeck's Connect dialog")
			}
			if toolkit == "" {
				return errors.New("say which toolkit: --toolkit <name>")
			}
			u := strings.TrimSuffix(base, "/") + "/mcp/" + toolkit
			if build {
				u += "/build"
			}
			return bridge(context.Background(), u, tok, os.Stdin, os.Stdout)
		}}
	c.Flags().StringVar(&toolkit, "toolkit", "", "toolkit name")
	c.Flags().StringVar(&base, "url", "http://127.0.0.1:47800", "mcpDeck address")
	c.Flags().BoolVar(&build, "build", false, "bridge the builder endpoint")
	return c
}

func bridge(ctx context.Context, endpoint, token string, in io.Reader, out io.Writer) error {
	var mu sync.Mutex
	write := func(b []byte) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = out.Write(append(bytes.TrimSpace(b), '\n'))
	}
	hc := &http.Client{}
	var session string
	var sseOnce sync.Once
	sse := func(body io.Reader) {
		sc := bufio.NewScanner(body)
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		var data []byte
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "data:"):
				data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:"))...)
			case line == "" && len(data) > 0:
				write(data)
				data = nil
			}
		}
		if len(data) > 0 {
			write(data)
		}
	}
	listen := func() {
		// Server-sent notifications such as tools/list_changed.
		for ctx.Err() == nil {
			req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Mcp-Session-Id", session)
			res, err := hc.Do(req)
			if err != nil || res.StatusCode >= 400 {
				if res != nil {
					res.Body.Close()
				}
				time.Sleep(2 * time.Second)
				continue
			}
			sse(res.Body)
			res.Body.Close()
		}
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(line))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		res, err := hc.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mcpdeck stdio: mcpDeck isn't reachable at", endpoint, "- start it with: mcpdeck")
			continue
		}
		if s := res.Header.Get("Mcp-Session-Id"); s != "" && session == "" {
			session = s
			sseOnce.Do(func() { go listen() })
		}
		ct := res.Header.Get("Content-Type")
		switch {
		case res.StatusCode >= 400:
			b, _ := io.ReadAll(res.Body)
			fmt.Fprintf(os.Stderr, "mcpdeck stdio: %s %s\n", res.Status, bytes.TrimSpace(b))
		case strings.HasPrefix(ct, "text/event-stream"):
			sse(res.Body)
		case strings.HasPrefix(ct, "application/json"):
			b, _ := io.ReadAll(res.Body)
			write(b)
		}
		res.Body.Close()
	}
	return sc.Err()
}
