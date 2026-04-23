package client

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	adminui "github.com/GizClaw/gizclaw-go/ui/apps/admin"
	playui "github.com/GizClaw/gizclaw-go/ui/apps/play"
)

func ListenAndServeAdminUI(ctxName, addr string, out io.Writer) error {
	return listenAndServeUI(ctxName, addr, "GizClaw Admin UI", adminui.FS(), out)
}

func ListenAndServePlayUI(ctxName, addr string, out io.Writer) error {
	return listenAndServeUI(ctxName, addr, "GizClaw Play UI", playui.FS(), out)
}

func listenAndServeUI(ctxName, addr, title string, uiFS fs.FS, out io.Writer) error {
	if strings.TrimSpace(addr) == "" {
		return fmt.Errorf("gizclaw: empty listen addr")
	}
	c, err := ConnectFromContext(ctxName)
	if err != nil {
		return err
	}
	defer c.Close()

	listener, err := net.Listen("tcp", normalizeListenAddr(addr))
	if err != nil {
		return fmt.Errorf("gizclaw: listen ui: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", c.ProxyHandler())
	mux.Handle("/api", c.ProxyHandler())
	mux.Handle("/", staticWithSPAFallback(uiFS))

	server := &http.Server{
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}

	if out != nil {
		_, _ = fmt.Fprintf(out, "%s listening on %s\n", title, displayURL(listener.Addr()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	err = server.Serve(listener)
	if err == nil || err == http.ErrServerClosed {
		return nil
	}
	return err
}

// staticWithSPAFallback serves embedded UI assets and falls back to index.html
// for client-side routes (e.g. /devices/...) so BrowserRouter deep links work.
func staticWithSPAFallback(uiFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(uiFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean != "" {
			if _, err := fs.Stat(uiFS, clean); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r2 := r.Clone(r.Context())
		r2.URL = r.URL
		u := *r.URL
		u.Path = "/"
		r2.URL = &u
		fileServer.ServeHTTP(w, r2)
	})
}

func normalizeListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return addr
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	return ":" + addr
}

func displayURL(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
