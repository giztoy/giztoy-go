package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	adminui "github.com/GizClaw/gizclaw-go/ui/apps/admin"
	playui "github.com/GizClaw/gizclaw-go/ui/apps/play"
	"github.com/pion/webrtc/v4"
)

func ListenAndServeAdminUI(ctxName, addr string, out io.Writer) error {
	return listenAndServeUI(ctxName, addr, "GizClaw Admin UI", adminui.FS(), out, nil, nil)
}

func ListenAndServePlayUI(ctxName, addr string, out io.Writer) error {
	return listenAndServeUI(ctxName, addr, "GizClaw Play UI", playui.FS(), out, ensurePlayRegistration, registerPlayUIRoutes)
}

func listenAndServeUI(
	ctxName, addr, title string,
	uiFS fs.FS,
	out io.Writer,
	beforeServe func(context.Context, *gizclaw.Client) error,
	registerRoutes func(*http.ServeMux, *gizclaw.Client),
) error {
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
	if beforeServe != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := beforeServe(ctx, c); err != nil {
			_ = listener.Close()
			return err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", c.ProxyHandler())
	mux.Handle("/api", c.ProxyHandler())
	if registerRoutes != nil {
		registerRoutes(mux, c)
	}
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

func ensurePlayRegistration(ctx context.Context, c *gizclaw.Client) error {
	gearAPI, err := c.GearServiceClient()
	if err != nil {
		return err
	}
	registration, err := gearAPI.GetRegistrationWithResponse(ctx)
	if err != nil {
		return err
	}
	if registration.JSON200 != nil {
		return nil
	}
	if registration.StatusCode() != http.StatusNotFound {
		return responseError(registration.StatusCode(), registration.Body, registration.JSON404)
	}

	publicAPI, err := c.ServerPublicClient()
	if err != nil {
		return err
	}
	created, err := publicAPI.RegisterGearWithResponse(ctx, serverpublic.RegistrationRequest{})
	if err != nil {
		return err
	}
	if created.JSON200 != nil || created.StatusCode() == http.StatusConflict {
		return nil
	}
	return responseError(created.StatusCode(), created.Body, created.JSON400, created.JSON409)
}

type playWebRTCOfferRequest struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

type playWebRTCAnswerResponse struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

func registerPlayUIRoutes(mux *http.ServeMux, c *gizclaw.Client) {
	mux.HandleFunc("/webrtc/offer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		handlePlayWebRTCOffer(w, r, c)
	})
}

func handlePlayWebRTCOffer(w http.ResponseWriter, r *http.Request, c *gizclaw.Client) {
	var req playWebRTCOfferRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid offer json", http.StatusBadRequest)
		return
	}
	if req.Type != webrtc.SDPTypeOffer.String() || strings.TrimSpace(req.SDP) == "" {
		http.Error(w, "invalid webrtc offer", http.StatusBadRequest)
		return
	}
	if c == nil {
		playWebRTCError(w, "client is not available", fmt.Errorf("nil client"), http.StatusServiceUnavailable)
		return
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		playWebRTCError(w, "create peer connection failed", err, http.StatusInternalServerError)
		return
	}
	registration, err := c.RegisterTo(pc)
	if err != nil {
		_ = pc.Close()
		playWebRTCError(w, "register peer connection failed", err, http.StatusInternalServerError)
		return
	}
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateDisconnected,
			webrtc.PeerConnectionStateClosed:
			_ = registration.Close()
			_ = pc.Close()
		}
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  req.SDP,
	}); err != nil {
		_ = registration.Close()
		_ = pc.Close()
		playWebRTCError(w, "set remote description failed", err, http.StatusBadRequest)
		return
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = registration.Close()
		_ = pc.Close()
		playWebRTCError(w, "create answer failed", err, http.StatusInternalServerError)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = registration.Close()
		_ = pc.Close()
		playWebRTCError(w, "set local description failed", err, http.StatusInternalServerError)
		return
	}
	select {
	case <-gatherComplete:
	case <-r.Context().Done():
		_ = registration.Close()
		_ = pc.Close()
		return
	}

	local := pc.LocalDescription()
	if local == nil {
		_ = registration.Close()
		_ = pc.Close()
		playWebRTCError(w, "missing local description", fmt.Errorf("local description is nil"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(playWebRTCAnswerResponse{SDP: local.SDP, Type: local.Type.String()}); err != nil {
		_ = registration.Close()
		_ = pc.Close()
	}
}

func playWebRTCError(w http.ResponseWriter, message string, err error, status int) {
	slog.Error("gizclaw: play webrtc signaling failed", "message", message, "error", err, "status", status)
	http.Error(w, fmt.Sprintf("%s: %v", message, err), status)
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
