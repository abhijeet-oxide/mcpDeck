package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// client calls a running mcpDeck's HTTP API (RUN-6).
type client struct {
	base  string
	token string
	agent string // set by the admin MCP server; actions are recorded as this agent's
	// timeout for one request; 0 is 60 seconds. Sign-in and recording wait for a person.
	timeout time.Duration
}

func localClient(data string) (*client, error) {
	if data == "" {
		data = dataDir()
	}
	in, err := readInstance(data)
	if err != nil {
		return nil, errors.New("mcpDeck isn't running. Start it with: mcpdeck")
	}
	return &client{base: "http://127.0.0.1:" + strconv.Itoa(in.Port), token: in.CLIToken}, nil
}

func (c *client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	if c.agent != "" {
		req.Header.Set("X-Mcpdeck-Agent", c.agent)
	}
	timeout := c.timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	res, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return errors.New("mcpDeck isn't running. Start it with: mcpdeck")
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		var e struct{ Error string }
		if json.Unmarshal(b, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("mcpDeck returned %s", res.Status)
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}
