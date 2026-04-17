package gizclaw

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/errgroup"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/rpc"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/firmware"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/giznet/gizhttp"
)

const (
	ServiceRPC          uint64 = 0x00
	ServiceServerPublic uint64 = 0x01
	ServicePeerPublic   uint64 = 0x02
	ServiceAdmin        uint64 = 0x10
	ServiceGear         uint64 = 0x11
)

type adminService struct {
	firmware.FirmwareAdminService
	gear.GearsAdminService
}

type gearService struct {
	firmware.FirmwareGearService
	gear.GearsGearService
}

type serverPublic struct {
	firmware.FirmwareServerPublic
	gear.GearsServerPublic
}

// Service serves one peer connection.
type Service struct {
	admin  *adminService
	gear   *gearService
	public *serverPublic
	rpc    *rpc.Server

	manager *Manager
}

var _ adminservice.StrictServerInterface = (*adminService)(nil)
var _ gearservice.StrictServerInterface = (*gearService)(nil)
var _ serverpublic.StrictServerInterface = (*serverPublic)(nil)

func (s *Service) ServeConn(conn *giznet.Conn) error {
	if s == nil {
		return errors.New("gizclaw: nil service")
	}
	if conn == nil {
		return errors.New("gizclaw: nil conn")
	}
	defer func() {
		_ = conn.Close()
	}()
	if err := s.validateServices(); err != nil {
		return err
	}
	s.markPeerOnline(conn)
	defer s.markPeerOffline(conn)

	var g errgroup.Group
	g.Go(func() error { return s.serveAdmin(conn) })
	g.Go(func() error { return s.serveGear(conn) })
	g.Go(func() error { return s.servePublic(conn) })
	g.Go(func() error { return s.serveRPC(conn) })

	return g.Wait()
}

func (s *Service) validateServices() error {
	switch {
	case s.admin == nil:
		return errors.New("gizclaw: nil admin service")
	case s.gear == nil:
		return errors.New("gizclaw: nil gear service")
	case s.public == nil:
		return errors.New("gizclaw: nil public service")
	case s.rpc == nil:
		return errors.New("gizclaw: nil rpc server")
	default:
		return nil
	}
}

func (s *Service) servePublic(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		base := ctx.UserContext()
		if base == nil {
			base = context.Background()
		}
		ctx.SetUserContext(serverpublic.WithCallerPublicKey(base, conn.PublicKey().String()))
		return ctx.Next()
	})
	serverpublic.RegisterHandlers(app, serverpublic.NewStrictHandler(s.public, nil))

	server := gizhttp.NewServer(conn, ServiceServerPublic, fiberHTTPHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *Service) serveAdmin(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		return ctx.Next()
	})
	handler := adminservice.NewStrictHandler(s.admin, nil)
	adminservice.RegisterHandlers(app, handler)

	server := gizhttp.NewServer(conn, ServiceAdmin, fiberHTTPHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *Service) serveGear(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		return ctx.Next()
	})
	handler := gearservice.NewStrictHandler(s.gear, nil)
	gearservice.RegisterHandlers(app, handler)

	server := gizhttp.NewServer(conn, ServiceGear, fiberHTTPHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *Service) serveRPC(conn *giznet.Conn) error {
	listener := conn.ListenService(ServiceRPC)
	defer func() {
		_ = listener.Close()
	}()
	for {
		stream, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.touchPeer(conn)

		go func(stream net.Conn) {
			defer stream.Close()
			for {
				err := s.rpc.Serve(stream)
				if err == nil {
					continue
				}
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
					return
				}
				return
			}
		}(stream)
	}
}

// fiberHTTPHandler adapts a Fiber app to net/http for gizhttp.NewServer.
func fiberHTTPHandler(app *fiber.App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		if r.Body != nil {
			n, err := io.Copy(req.BodyWriter(), r.Body)
			req.Header.SetContentLength(int(n))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
		req.Header.SetMethod(r.Method)
		req.SetRequestURI(r.RequestURI)
		req.SetHost(r.Host)
		req.Header.SetHost(r.Host)
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		remoteAddr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			remoteAddr, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		}

		var fctx fasthttp.RequestCtx
		fctx.Init(req, remoteAddr, nil)
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					fctx.Response.Reset()
					fctx.Response.SetStatusCode(http.StatusInternalServerError)
					fctx.Response.SetBodyString(http.StatusText(http.StatusInternalServerError))
				}
			}()
			app.Handler()(&fctx)
		}()

		responseBody := fctx.Response.Body()
		fctx.Response.Header.VisitAll(func(k, v []byte) {
			w.Header().Add(string(k), string(v))
		})
		if len(responseBody) > 0 && w.Header().Get("Content-Length") == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
		}
		statusCode := fctx.Response.StatusCode()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write(responseBody)
	})
}

func (s *Service) markPeerOnline(conn *giznet.Conn) {
	if s == nil || s.manager == nil || conn == nil {
		return
	}
	s.manager.MarkPeerOnline(conn.PublicKey().String(), conn)
}

func (s *Service) markPeerOffline(conn *giznet.Conn) {
	if s == nil || s.manager == nil || conn == nil {
		return
	}
	s.manager.MarkPeerOffline(conn.PublicKey().String(), conn)
}

func (s *Service) touchPeer(conn *giznet.Conn) {
	if s == nil || s.manager == nil || conn == nil {
		return
	}
	s.manager.TouchPeer(conn.PublicKey().String(), conn)
}
