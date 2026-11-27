package browser

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6455 fixes SHA-1 for the handshake accept key
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// wsConn is the small part of RFC 6455 the DevTools protocol needs: one
// client connection to 127.0.0.1, text frames, ping and close. It keeps
// mcpDeck free of a websocket dependency.
type wsConn struct {
	c  net.Conn
	r  *bufio.Reader
	mu sync.Mutex // one writer at a time
}

func dialWS(rawURL string, timeout time.Duration) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("the browser's debugging URL %q isn't ws://", rawURL)
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, fmt.Errorf("the browser's debugging URL must be on this computer, not %s", host)
	}
	c, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	k := base64.StdEncoding.EncodeToString(key)
	req := "GET " + u.RequestURI() + " HTTP/1.1\r\nHost: " + u.Host + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: " + k + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	_ = c.SetDeadline(time.Now().Add(timeout))
	if _, err := io.WriteString(c, req); err != nil {
		_ = c.Close()
		return nil, err
	}
	r := bufio.NewReaderSize(c, 64<<10)
	res, err := http.ReadResponse(r, &http.Request{Method: http.MethodGet})
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		_ = c.Close()
		return nil, fmt.Errorf("the browser refused the debugging connection: %s", res.Status)
	}
	h := sha1.Sum([]byte(k + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")) //nolint:gosec // RFC 6455
	if res.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(h[:]) {
		_ = c.Close()
		return nil, errors.New("the browser's debugging connection answered with a wrong accept key")
	}
	_ = c.SetDeadline(time.Time{})
	return &wsConn{c: c, r: r}, nil
}

func (w *wsConn) writeFrame(op byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	hdr := []byte{0x80 | op}
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n <= 0xFFFF:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		hdr = append(hdr, b[:]...)
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	hdr = append(hdr, mask...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := w.c.Write(append(hdr, masked...))
	return err
}

// WriteText sends one text message.
func (w *wsConn) WriteText(b []byte) error { return w.writeFrame(0x1, b) }

// ReadMessage returns the next text or binary message, answering pings.
func (w *wsConn) ReadMessage() ([]byte, error) {
	var msg []byte
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.r, h[:]); err != nil {
			return nil, err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0F
		n := uint64(h[1] & 0x7F)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		if n > 64<<20 {
			return nil, errors.New("a debugging message over 64 MB")
		}
		var mask []byte
		if h[1]&0x80 != 0 {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(w.r, mask); err != nil {
				return nil, err
			}
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(w.r, p); err != nil {
			return nil, err
		}
		if mask != nil {
			for i := range p {
				p[i] ^= mask[i%4]
			}
		}
		switch op {
		case 0x8: // close
			_ = w.writeFrame(0x8, nil)
			return nil, io.EOF
		case 0x9: // ping
			_ = w.writeFrame(0xA, p)
			continue
		case 0xA: // pong
			continue
		}
		msg = append(msg, p...)
		if fin {
			return msg, nil
		}
	}
}

func (w *wsConn) Close() error { return w.c.Close() }
