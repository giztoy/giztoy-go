package gizclaw

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/errgroup"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/credential"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/firmware"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/mmx"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/publiclogin"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/resourcemanager"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/workspace"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/workspacetemplate"
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
	credential.CredentialAdminService
	firmware.FirmwareAdminService
	gear.GearsAdminService
	mmx.MiniMaxAdminService
	workspace.WorkspaceAdminService
	workspacetemplate.WorkspaceTemplateAdminService
	ResourceManager *resourcemanager.Manager
}

func (s *adminService) ApplyResource(ctx context.Context, request adminservice.ApplyResourceRequestObject) (adminservice.ApplyResourceResponseObject, error) {
	if request.JSONBody == nil {
		return adminservice.ApplyResource400JSONResponse(apitypes.NewErrorResponse("INVALID_RESOURCE", "request body is required")), nil
	}
	result, err := s.ResourceManager.Apply(ctx, *request.JSONBody)
	if err != nil {
		status, body := resourceManagerError(err)
		switch status {
		case http.StatusBadRequest:
			return adminservice.ApplyResource400JSONResponse(body), nil
		case http.StatusConflict:
			return adminservice.ApplyResource409JSONResponse(body), nil
		default:
			return adminservice.ApplyResource500JSONResponse(body), nil
		}
	}
	return adminservice.ApplyResource200JSONResponse(result), nil
}

func (s *adminService) GetResource(ctx context.Context, request adminservice.GetResourceRequestObject) (adminservice.GetResourceResponseObject, error) {
	resource, err := s.ResourceManager.Get(ctx, request.Kind, request.Name)
	if err != nil {
		status, body := resourceManagerError(err)
		switch status {
		case http.StatusBadRequest:
			return adminservice.GetResource400JSONResponse(body), nil
		case http.StatusNotFound:
			return adminservice.GetResource404JSONResponse(body), nil
		default:
			return adminservice.GetResource500JSONResponse(body), nil
		}
	}
	return resource200JSONResponse{Resource: resource}, nil
}

func (s *adminService) PutResource(ctx context.Context, request adminservice.PutResourceRequestObject) (adminservice.PutResourceResponseObject, error) {
	if request.JSONBody == nil {
		return adminservice.PutResource400JSONResponse(apitypes.NewErrorResponse("INVALID_RESOURCE", "request body is required")), nil
	}
	if err := validateResourcePathMatch(*request.JSONBody, request.Kind, request.Name); err != nil {
		return adminservice.PutResource400JSONResponse(apitypes.NewErrorResponse("INVALID_RESOURCE_PATH", err.Error())), nil
	}
	resource, err := s.ResourceManager.Put(ctx, *request.JSONBody)
	if err != nil {
		status, body := resourceManagerError(err)
		switch status {
		case http.StatusBadRequest:
			return adminservice.PutResource400JSONResponse(body), nil
		case http.StatusNotFound:
			return adminservice.PutResource404JSONResponse(body), nil
		case http.StatusConflict:
			return adminservice.PutResource409JSONResponse(body), nil
		default:
			return adminservice.PutResource500JSONResponse(body), nil
		}
	}
	return resource200JSONResponse{Resource: resource}, nil
}

func (s *adminService) DeleteResource(ctx context.Context, request adminservice.DeleteResourceRequestObject) (adminservice.DeleteResourceResponseObject, error) {
	resource, err := s.ResourceManager.Delete(ctx, request.Kind, request.Name)
	if err != nil {
		status, body := resourceManagerError(err)
		switch status {
		case http.StatusBadRequest:
			return adminservice.DeleteResource400JSONResponse(body), nil
		case http.StatusNotFound:
			return adminservice.DeleteResource404JSONResponse(body), nil
		case http.StatusConflict:
			return adminservice.DeleteResource409JSONResponse(body), nil
		default:
			return adminservice.DeleteResource500JSONResponse(body), nil
		}
	}
	return resource200JSONResponse{Resource: resource}, nil
}

type resource200JSONResponse struct {
	Resource apitypes.Resource
}

func (response resource200JSONResponse) VisitGetResourceResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(http.StatusOK)
	return ctx.JSON(&response.Resource)
}

func (response resource200JSONResponse) VisitPutResourceResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(http.StatusOK)
	return ctx.JSON(&response.Resource)
}

func (response resource200JSONResponse) VisitDeleteResourceResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(http.StatusOK)
	return ctx.JSON(&response.Resource)
}

func resourceManagerError(err error) (int, apitypes.ErrorResponse) {
	var resourceErr *resourcemanager.Error
	if errors.As(err, &resourceErr) {
		return resourceErr.StatusCode, apitypes.NewErrorResponse(resourceErr.Code, resourceErr.Message)
	}
	return http.StatusInternalServerError, apitypes.NewErrorResponse("RESOURCE_MANAGER_ERROR", err.Error())
}

func validateResourcePathMatch(resource apitypes.Resource, kind apitypes.ResourceKind, name string) error {
	var header struct {
		Kind     apitypes.ResourceKind `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	data, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.Kind != kind {
		return errors.New("resource kind does not match path kind")
	}
	if header.Metadata.Name != name {
		return errors.New("resource metadata.name does not match path name")
	}
	return nil
}

type gearAPIBundle struct {
	firmware.FirmwareGearService
	gear.GearsGearService
}

type serverPublic struct {
	gear.GearsServerPublic
	publiclogin.ServerPublic
}

// PeerService serves one peer connection.
type PeerService struct {
	admin       *adminService
	gear        *gearAPIBundle
	public      *serverPublic
	peerManager *Manager
}

var _ adminservice.StrictServerInterface = (*adminService)(nil)
var _ gearservice.StrictServerInterface = (*gearAPIBundle)(nil)
var _ serverpublic.StrictServerInterface = (*serverPublic)(nil)

func (s *PeerService) ServeConn(conn *giznet.Conn) error {
	if s == nil {
		return errors.New("gizclaw: nil peer service")
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
	if err := s.ensurePeerGear(context.Background(), conn); err != nil {
		return err
	}
	s.markPeerOnline(conn)
	defer s.markPeerOffline(conn)

	var g errgroup.Group
	g.Go(func() error { return s.serveAdmin(conn) })
	g.Go(func() error { return s.serveGear(conn) })
	g.Go(func() error { return s.servePublic(conn) })

	return g.Wait()
}

func (s *PeerService) ensurePeerGear(ctx context.Context, conn *giznet.Conn) error {
	if s == nil || s.peerManager == nil {
		return nil
	}
	_, err := s.peerManager.EnsurePeerGear(ctx, conn.PublicKey().String())
	return err
}

func (s *PeerService) validateServices() error {
	switch {
	case s.admin == nil:
		return errors.New("gizclaw: nil admin service")
	case s.gear == nil:
		return errors.New("gizclaw: nil gear service")
	case s.public == nil:
		return errors.New("gizclaw: nil public service")
	default:
		return nil
	}
}

func (s *PeerService) servePublic(conn *giznet.Conn) error {
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

func (s *PeerService) serveAdmin(conn *giznet.Conn) error {
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

func (s *PeerService) serveGear(conn *giznet.Conn) error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		s.touchPeer(conn)
		base := ctx.UserContext()
		if base == nil {
			base = context.Background()
		}
		ctx.SetUserContext(gearservice.WithCallerPublicKey(base, conn.PublicKey().String()))
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
		requestURI := r.URL.RequestURI()
		if requestURI == "" {
			requestURI = r.RequestURI
		}
		req.SetRequestURI(requestURI)
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

func (s *PeerService) markPeerOnline(conn *giznet.Conn) {
	s.peerManager.MarkPeerOnline(conn.PublicKey().String(), conn)
}

func (s *PeerService) markPeerOffline(conn *giznet.Conn) {
	s.peerManager.MarkPeerOffline(conn.PublicKey().String(), conn)
}

func (s *PeerService) touchPeer(conn *giznet.Conn) {
	s.peerManager.TouchPeer(conn.PublicKey().String(), conn)
}
