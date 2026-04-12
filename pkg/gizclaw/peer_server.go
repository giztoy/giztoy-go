package gizclaw

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/errgroup"

	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/adminservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/gearservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/rpc"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/serverpublic"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/giznet/gizhttp"
)

// PeerServer serves one peer connection.
type PeerServer struct {
	Admin  adminservice.StrictServerInterface
	Gear   gearservice.StrictServerInterface
	Public serverpublic.StrictServerInterface
	RPC    *rpc.Server

	Manager *Manager
}

func (s *PeerServer) Serve(conn *giznet.Conn) error {
	if s == nil {
		return errors.New("gizclaw: nil peer server")
	}
	if conn == nil {
		return errors.New("gizclaw: nil conn")
	}
	if err := s.validateServices(); err != nil {
		return err
	}
	s.markPeerOnline(conn)
	defer s.markPeerOffline(conn)

	var g errgroup.Group

	g.Go(func() error {
		return s.serveAdmin(conn)
	})
	g.Go(func() error {
		return s.serveGear(conn)
	})
	g.Go(func() error {
		return s.servePublic(conn)
	})
	g.Go(func() error {
		return s.serveRPC(conn)
	})
	g.Go(func() error {
		return s.serveAction(conn)
	})

	return g.Wait()
}

func (s *PeerServer) validateServices() error {
	switch {
	case s.Admin == nil:
		return errors.New("gizclaw: nil admin service")
	case s.Gear == nil:
		return errors.New("gizclaw: nil gear service")
	case s.Public == nil:
		return errors.New("gizclaw: nil public service")
	case s.RPC == nil:
		return errors.New("gizclaw: nil rpc server")
	default:
		return nil
	}
}

func (s *PeerServer) servePublic(conn *giznet.Conn) error {
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
	serverpublic.RegisterHandlers(app, serverpublic.NewStrictHandler(s.Public, nil))

	server := gizhttp.NewServer(conn, ServiceServerPublic, fiberAppHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *PeerServer) serveAdmin(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		return ctx.Next()
	})
	adminHandler := adminservice.NewStrictHandler(s.Admin, nil)
	admin := adminservice.ServerInterfaceWrapper{Handler: adminHandler}
	app.Get("/firmwares", admin.ListDepots)
	app.Put("/firmwares/*", func(ctx *fiber.Ctx) error {
		path := strings.TrimPrefix(ctx.Path(), "/firmwares/")
		switch {
		case strings.HasSuffix(path, ":release"):
			return adminHandler.ReleaseDepot(ctx, adminservice.DepotName(strings.TrimSuffix(path, ":release")))
		case strings.HasSuffix(path, ":rollback"):
			return adminHandler.RollbackDepot(ctx, adminservice.DepotName(strings.TrimSuffix(path, ":rollback")))
		default:
			return ctx.Next()
		}
	})
	app.Get("/firmwares/:depot", admin.GetDepot)
	app.Put("/firmwares/:depot", admin.PutDepotInfo)
	app.Get("/firmwares/:depot/:channel", admin.GetChannel)
	app.Put("/firmwares/:depot/:channel", admin.PutChannel)

	server := gizhttp.NewServer(conn, ServiceAdmin, fiberAppHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *PeerServer) serveGear(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		return ctx.Next()
	})
	gearHandler := gearservice.NewStrictHandler(s.Gear, nil)
	gear := gearservice.ServerInterfaceWrapper{Handler: gearHandler}
	app.Get("/gears", gear.ListGears)
	app.Get("/gears/certification/:type/:authority/:id", gear.ListByCertification)
	app.Get("/gears/firmware/:depot/:channel", gear.ListByFirmware)
	app.Get("/gears/imei/:tac/:serial", gear.ResolveByIMEI)
	app.Get("/gears/label/:key/:value", gear.ListByLabel)
	app.Get("/gears/sn/:sn", gear.ResolveBySN)
	app.Delete("/gears/:publicKey", gear.DeleteGear)
	app.Get("/gears/:publicKey", gear.GetGear)
	app.Get("/gears/:publicKey/config", gear.GetGearConfig)
	app.Put("/gears/:publicKey/config", gear.PutGearConfig)
	app.Get("/gears/:publicKey/info", gear.GetGearInfo)
	app.Get("/gears/:publicKey/ota", gear.GetGearOTA)
	app.Get("/gears/:publicKey/runtime", gear.GetGearRuntime)
	app.Post("/gears/*", func(ctx *fiber.Ctx) error {
		path := strings.TrimPrefix(ctx.Path(), "/gears/")
		switch {
		case strings.HasSuffix(path, ":approve"):
			return gearHandler.ApproveGear(ctx, gearservice.PublicKey(strings.TrimSuffix(path, ":approve")))
		case strings.HasSuffix(path, ":block"):
			return gearHandler.BlockGear(ctx, gearservice.PublicKey(strings.TrimSuffix(path, ":block")))
		case strings.HasSuffix(path, ":refresh"):
			return gearHandler.RefreshGear(ctx, gearservice.PublicKey(strings.TrimSuffix(path, ":refresh")))
		default:
			return fiber.ErrNotFound
		}
	})

	server := gizhttp.NewServer(conn, ServiceGear, fiberAppHandler(app))
	defer func() {
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *PeerServer) serveRPC(conn *giznet.Conn) error {
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
			_ = s.RPC.Serve(stream)
		}(stream)
	}
}

func (s *PeerServer) serveAction(_ *giznet.Conn) error {
	return nil
}

func fiberAppHandler(app *fiber.App) http.Handler {
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

func (s *PeerServer) markPeerOnline(conn *giznet.Conn) {
	if s == nil || s.Manager == nil || conn == nil {
		return
	}
	s.Manager.MarkPeerOnline(conn.PublicKey().String(), conn)
}

func (s *PeerServer) markPeerOffline(conn *giznet.Conn) {
	if s == nil || s.Manager == nil || conn == nil {
		return
	}
	s.Manager.MarkPeerOffline(conn.PublicKey().String(), conn)
}

func (s *PeerServer) touchPeer(conn *giznet.Conn) {
	if s == nil || s.Manager == nil || conn == nil {
		return
	}
	s.Manager.TouchPeer(conn.PublicKey().String(), conn)
}
