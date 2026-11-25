package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/clock"
	"github.com/abhijeet-oxide/mcpdeck/internal/connect"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/mock"
	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
	"github.com/abhijeet-oxide/mcpdeck/internal/server"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/vault"
	"github.com/abhijeet-oxide/mcpdeck/web"
)

type serveOpts struct {
	data       string
	port       int
	noOpen     bool
	dev        bool
	mockPort   int
	launchFile string
	returnURL  string
}

const defaultPort = 47800

func uiFS() fs.FS {
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}

// alreadyRunning opens the browser on a running instance (RUN-2).
func alreadyRunning(dir string, open bool) bool {
	in, err := readInstance(dir)
	if err != nil {
		return false
	}
	c := &client{base: "http://127.0.0.1:" + strconv.Itoa(in.Port), token: in.CLIToken}
	var out struct{ URL string }
	if err := c.do("POST", "/api/launch-url", nil, &out); err != nil {
		return false
	}
	fmt.Printf("mcpDeck is already running at http://127.0.0.1:%d\n", in.Port)
	if open {
		_ = opener{}.Open(out.URL)
	} else {
		fmt.Println("Open:", out.URL)
	}
	return true
}

func serve(o serveOpts) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if o.data == "" {
		o.data = dataDir()
	}
	if err := os.MkdirAll(o.data, 0o700); err != nil {
		return fmt.Errorf("can't create the data folder %s: %w", o.data, err)
	}
	if !o.dev && alreadyRunning(o.data, !o.noOpen) {
		return nil
	}
	st, err := store.Open(ctx, o.data)
	if err != nil {
		return err
	}
	defer st.Close()
	// The port is stored on first run so client configs stay valid (RUN-4).
	if o.port == 0 {
		o.port, _ = strconv.Atoi(st.Setting(ctx, "port", strconv.Itoa(defaultPort)))
	}
	if !o.dev {
		_ = st.SetSetting(ctx, "port", strconv.Itoa(o.port))
	}
	ln, err := net.Listen("tcp", server.Addr(o.port))
	if err != nil {
		return fmt.Errorf("port %d is in use by another program. Start mcpDeck with --port <number>, then reconnect your agents", o.port)
	}
	var kc vault.Keychain = vault.OSKeychain{}
	if o.dev {
		kc = vault.NoKeychain{}
	}
	v, err := vault.Open(kc, o.data)
	if err != nil {
		return err
	}
	c := clock.Real{}
	ids := clock.ULIDs{Clock: c}
	d := deck.New(st, v, proxy.New(st, v, c, ids, nil), audit.New(st, c, ids), c, ids, o.data, o.port)
	d.Demo = o.dev

	var world *mock.World
	if o.dev {
		world, err = mock.Start(o.mockPort)
		if err != nil {
			return err
		}
		defer world.Close()
		if apis, _ := st.APIs(ctx); len(apis) == 0 {
			if err := mock.Seed(ctx, d, world, true); err != nil {
				return err
			}
		}
	}

	g := server.NewGuard(o.port)
	httpSrv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	var demoMu sync.Mutex
	var demoCmd *exec.Cmd
	opts := server.Options{Deck: d, Guard: g, UI: uiFS(), OS: connect.RealOS{}, Opener: opener{}, ReturnURL: o.returnURL,
		Shutdown: func() { stop() }}
	if !o.dev {
		// The demo runs as its own instance with temporary data (UI-12).
		opts.StartDemo = func(context.Context) (string, error) {
			demoMu.Lock()
			defer demoMu.Unlock()
			tmp, err := os.MkdirTemp("", "mcpdeck-demo-")
			if err != nil {
				return "", err
			}
			lf := filepath.Join(tmp, "launch.url")
			exe, _ := os.Executable()
			if demoCmd != nil && demoCmd.Process != nil {
				_ = demoCmd.Process.Kill()
			}
			demoCmd = exec.Command(exe, "dev", "--port", strconv.Itoa(o.port+1), "--mock-port", "0", "--data", tmp, "--no-open",
				"--launch-file", lf, "--return-url", "http://127.0.0.1:"+strconv.Itoa(o.port)+"/")
			if err := demoCmd.Start(); err != nil {
				return "", err
			}
			for i := 0; i < 100; i++ {
				if b, err := os.ReadFile(lf); err == nil && len(b) > 0 {
					return strings.TrimSpace(string(b)), nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			return "", errors.New("the demo didn't start; run mcpdeck dev in a terminal to see why")
		}
	}
	httpSrv.Handler = server.New(opts)
	if !o.dev {
		if err := writeInstance(o.data, instance{Port: o.port, PID: os.Getpid(), CLIToken: g.CLIToken}); err != nil {
			return err
		}
		defer os.Remove(instancePath(o.data))
	} else {
		_ = writeInstance(o.data, instance{Port: o.port, PID: os.Getpid(), CLIToken: g.CLIToken})
	}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			stop()
		}
	}()
	// Retention (SES-5), hourly.
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			_ = d.Prune(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	launch := g.LaunchURL("/")
	base := "http://127.0.0.1:" + strconv.Itoa(o.port)
	if o.dev {
		fmt.Printf("mcpDeck dev mode · temporary data in %s\n\n", o.data)
		fmt.Printf("  UI          %s\n", launch)
		fmt.Printf("  dev toolkit %s/mcp/dev   (Authorization: Bearer %s)\n", base, mock.DevToken)
		fmt.Printf("  builder     %s/mcp/dev/build\n\n", base)
		for _, n := range []string{"crm", "billing", "support", "legacy"} {
			fmt.Printf("  mock %-8s %s\n", n, world.URL(n))
		}
		fmt.Println()
	} else {
		fmt.Printf("mcpDeck is running at %s\n", base)
		fmt.Printf("Open: %s\n", launch)
		fmt.Println("Closing this window stops mcpDeck.")
	}
	if o.launchFile != "" {
		_ = os.WriteFile(o.launchFile, []byte(launch), 0o600)
	}
	if !o.noOpen {
		_ = opener{}.Open(launch)
	}
	<-ctx.Done()
	demoMu.Lock()
	if demoCmd != nil && demoCmd.Process != nil {
		_ = demoCmd.Process.Kill()
	}
	demoMu.Unlock()
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(sctx)
	return nil
}
