package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type event struct {
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	SessionID string          `json:"sessionId"`
}

type reply struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
	event
}

// Conn is one DevTools protocol connection to the browser.
type Conn struct {
	ws      *wsConn
	mu      sync.Mutex
	nextID  int64
	waiting map[int64]chan reply
	onEvent func(event) // called on the read goroutine; set with handle
	done    chan struct{}
}

// handle sets the event handler; next receives the handler it replaces.
func (c *Conn) handle(fn func(prev func(event)) func(event)) {
	c.mu.Lock()
	c.onEvent = fn(c.onEvent)
	c.mu.Unlock()
}

func newConn(ws *wsConn) *Conn {
	c := &Conn{ws: ws, waiting: map[int64]chan reply{}, done: make(chan struct{}), onEvent: func(event) {}}
	go c.read()
	return c
}

func (c *Conn) read() {
	defer close(c.done)
	for {
		b, err := c.ws.ReadMessage()
		if err != nil {
			c.mu.Lock()
			for id, ch := range c.waiting {
				close(ch)
				delete(c.waiting, id)
			}
			c.mu.Unlock()
			return
		}
		var r reply
		if json.Unmarshal(b, &r) != nil {
			continue
		}
		if r.ID != 0 {
			c.mu.Lock()
			ch := c.waiting[r.ID]
			delete(c.waiting, r.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- r
			}
			continue
		}
		c.mu.Lock()
		on := c.onEvent
		c.mu.Unlock()
		on(r.event)
	}
}

// Call sends one command and waits for its result.
func (c *Conn) Call(ctx context.Context, method string, params any, sessionID string) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan reply, 1)
	c.waiting[id] = ch
	c.mu.Unlock()
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if sessionID != "" {
		msg["sessionId"] = sessionID
	}
	b, _ := json.Marshal(msg)
	if err := c.ws.WriteText(b); err != nil {
		return nil, err
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, ErrClosed
		}
		if r.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, r.Error.Message)
		}
		return r.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, errors.New(method + ": the browser didn't answer within 30 seconds")
	}
}

// Close ends the connection.
func (c *Conn) Close() { _ = c.ws.Close() }

// Page is one tab, for tests that act as the person.
type Page struct {
	c       *Conn
	session string
}

// Eval runs JavaScript in the page and returns its JSON value.
func (p *Page) Eval(ctx context.Context, js string) (json.RawMessage, error) {
	res, err := p.c.Call(ctx, "Runtime.evaluate", map[string]any{"expression": js, "returnByValue": true, "awaitPromise": true}, p.session)
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	_ = json.Unmarshal(res, &r)
	return r.Result.Value, nil
}

// Close closes the tab.
func (p *Page) Close(ctx context.Context) error {
	_, err := p.c.Call(ctx, "Page.close", nil, p.session)
	return err
}
