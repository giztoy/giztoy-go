package gizclaw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/peerpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/rpc"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/giznet/gizhttp"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

var _ peerpublic.StrictServerInterface = (*Client)(nil)

// Client holds device-side peer client configuration.
type Client struct {
	KeyPair *giznet.KeyPair

	Device apitypes.DeviceInfo

	mu       sync.RWMutex
	listener *giznet.Listener
	conn     *giznet.Conn
	serverPK giznet.PublicKey
}

// DialAndServe establishes the peer connection and serves peer public handlers.
func (c *Client) DialAndServe(serverPK giznet.PublicKey, serverAddr string, opts ...giznet.Option) error {
	if c == nil {
		return fmt.Errorf("gizclaw: nil client")
	}
	if c.KeyPair == nil {
		return fmt.Errorf("gizclaw: nil key pair")
	}
	if serverAddr == "" {
		return fmt.Errorf("gizclaw: empty server addr")
	}
	c.mu.RLock()
	alreadyStarted := c.listener != nil || c.conn != nil
	c.mu.RUnlock()
	if alreadyStarted {
		return fmt.Errorf("gizclaw: client already started")
	}

	all := append([]giznet.Option{
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithServiceMuxConfig(giznet.ServiceMuxConfig{
			OnNewService: func(_ giznet.PublicKey, service uint64) bool {
				return service == ServiceServerPublic || service == ServicePeerPublic || service == ServiceRPC
			},
		}),
	}, opts...)
	l, err := giznet.Listen(c.KeyPair, all...)
	if err != nil {
		return fmt.Errorf("gizclaw: listen: %w", err)
	}
	c.mu.Lock()
	c.listener = l
	c.serverPK = serverPK
	c.mu.Unlock()

	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		c.mu.Lock()
		if c.listener == l {
			c.listener = nil
			c.serverPK = giznet.PublicKey{}
		}
		c.mu.Unlock()
		_ = l.Close()
		return fmt.Errorf("gizclaw: resolve addr: %w", err)
	}

	conn, err := l.Dial(serverPK, udpAddr)
	if err != nil {
		c.mu.Lock()
		if c.listener == l {
			c.listener = nil
			c.serverPK = giznet.PublicKey{}
		}
		c.mu.Unlock()
		_ = l.Close()
		return fmt.Errorf("gizclaw: dial: %w", err)
	}
	c.mu.Lock()
	if c.listener != l {
		c.mu.Unlock()
		_ = conn.Close()
		_ = l.Close()
		return fmt.Errorf("gizclaw: client closed during dial")
	}
	c.conn = conn
	c.mu.Unlock()

	var g errgroup.Group
	g.Go(c.servePeerPublic)
	g.Go(c.serveRPC)
	g.Go(c.servePackets)
	if err := g.Wait(); err != nil {
		_ = c.Close()
		return err
	}
	return nil
}

// Close releases all resources including the underlying UDP socket.
func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	listener := c.listener
	c.conn = nil
	c.listener = nil
	c.serverPK = giznet.PublicKey{}
	c.mu.Unlock()

	var err error
	if conn != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if listener != nil {
		if closeErr := listener.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func (c *Client) GetInfo(_ context.Context, _ peerpublic.GetInfoRequestObject) (peerpublic.GetInfoResponseObject, error) {
	return peerpublic.GetInfo200JSONResponse(gearDeviceToPeerRefreshInfo(c.Device)), nil
}

func (c *Client) GetIdentifiers(_ context.Context, _ peerpublic.GetIdentifiersRequestObject) (peerpublic.GetIdentifiersResponseObject, error) {
	return peerpublic.GetIdentifiers200JSONResponse(gearDeviceToPeerRefreshIdentifiers(c.Device)), nil
}

func (c *Client) GetVersion(_ context.Context, _ peerpublic.GetVersionRequestObject) (peerpublic.GetVersionResponseObject, error) {
	return peerpublic.GetVersion200JSONResponse(gearDeviceToPeerRefreshVersion(c.Device)), nil
}

// HTTPClient returns an HTTP client bound to a peer service.
func (c *Client) HTTPClient(service uint64) *http.Client {
	return gizhttp.NewClient(c.PeerConn(), service)
}

func (c *Client) ServerAdminClient() (*adminservice.ClientWithResponses, error) {
	return adminservice.NewClientWithResponses(
		"http://gizclaw",
		adminservice.WithHTTPClient(c.HTTPClient(ServiceAdmin)),
	)
}

func (c *Client) GearServiceClient() (*gearservice.ClientWithResponses, error) {
	return gearservice.NewClientWithResponses(
		"http://gizclaw",
		gearservice.WithHTTPClient(c.HTTPClient(ServiceGear)),
	)
}

func (c *Client) ServerPublicClient() (*serverpublic.ClientWithResponses, error) {
	return serverpublic.NewClientWithResponses(
		"http://gizclaw",
		serverpublic.WithHTTPClient(c.HTTPClient(ServiceServerPublic)),
	)
}

func (c *Client) PeerPublicClient() (*peerpublic.ClientWithResponses, error) {
	return peerpublic.NewClientWithResponses(
		"http://gizclaw",
		peerpublic.WithHTTPClient(c.HTTPClient(ServicePeerPublic)),
	)
}

// Ping opens a fresh RPC stream, sends one ping, and closes it.
//
// Our current RPC transport uses one KCP stream per round trip so multiple RPC
// requests can run concurrently on separate streams. This is closer to
// HTTP/1.0-style request lifecycles; HTTP/1.1-style stream reuse is not
// supported yet.
func (c *Client) Ping(ctx context.Context, id string) (*rpc.PingResponse, error) {
	rpcClient, err := c.rpcClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rpcClient.Close() }()
	return rpcClient.Ping(ctx, id)
}

func (c *Client) rpcClient() (*rpc.Client, error) {
	conn := c.PeerConn()
	if conn == nil {
		return nil, fmt.Errorf("gizclaw: client is not connected")
	}
	stream, err := conn.Dial(ServiceRPC)
	if err != nil {
		return nil, fmt.Errorf("gizclaw: dial rpc stream: %w", err)
	}
	return rpc.NewClient(stream), nil
}

// PeerConn returns the underlying peer connection.
func (c *Client) PeerConn() *giznet.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// ServerPublicKey returns the expected remote server public key.
func (c *Client) ServerPublicKey() giznet.PublicKey {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverPK
}

// servePeerPublic runs the device-side peer public HTTP service on ServicePeerPublic.
func (c *Client) servePeerPublic() error {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	peerpublic.RegisterHandlers(app, peerpublic.NewStrictHandler(c, nil))

	server := gizhttp.NewServer(c.conn, ServicePeerPublic, fiberHTTPHandler(app))
	return server.Serve()
}

func (c *Client) serveRPC() error {
	listener := c.conn.ListenService(ServiceRPC)
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

		go func(stream net.Conn) {
			if err := c.serveRPCStream(stream); err != nil {
				_ = stream.Close()
			}
		}(stream)
	}
}

func (c *Client) serveRPCStream(stream net.Conn) error {
	req, err := rpc.ReadRequest(stream)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}

	resp, err := c.dispatchRPC(context.Background(), req)
	if err != nil {
		return err
	}
	if resp == nil {
		resp = &rpc.RPCResponse{V: 1, Id: req.Id}
	}
	if resp.Id == "" {
		resp.Id = req.Id
	}
	if resp.V == 0 {
		resp.V = 1
	}
	if err := rpc.WriteResponse(stream, resp); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) dispatchRPC(_ context.Context, req *rpc.RPCRequest) (*rpc.RPCResponse, error) {
	switch req.Method {
	case rpc.MethodPing:
		if req.Params == nil {
			return rpc.ErrorResponse(req.Id, -32602, "missing params"), nil
		}
		return rpc.ResultResponse(req.Id, &rpc.PingResponse{ServerTime: time.Now().UnixMilli()}), nil
	default:
		return rpc.ErrorResponse(req.Id, -1, fmt.Sprintf("unknown method: %s", req.Method)), nil
	}
}

func (c *Client) servePackets() error {
	return nil
}

// ProxyHandler exposes the local reverse-proxy routes for remote server APIs.
func (c *Client) ProxyHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/admin/", http.StripPrefix("/api/admin", c.proxyService(ServiceAdmin)))
	mux.Handle("/api/public/", http.StripPrefix("/api/public", c.proxyService(ServiceServerPublic)))
	mux.Handle("/api/gear/", http.StripPrefix("/api/gear", c.proxyService(ServiceGear)))
	mux.HandleFunc("/api/admin", redirectProxyPrefix("/api/admin/"))
	mux.HandleFunc("/api/public", redirectProxyPrefix("/api/public/"))
	mux.HandleFunc("/api/gear", redirectProxyPrefix("/api/gear/"))
	mux.HandleFunc("/api", redirectProxyPrefix("/api/"))
	return mux
}

func (c *Client) proxyService(service uint64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c == nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		conn := c.PeerConn()
		if conn == nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		newServiceProxy(conn, service).ServeHTTP(w, r)
	})
}

func newServiceProxy(conn *giznet.Conn, service uint64) *httputil.ReverseProxy {
	target := &url.URL{
		Scheme: "http",
		Host:   "gizclaw",
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = gizhttp.NewRoundTripper(conn, service)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}
	return proxy
}

func redirectProxyPrefix(target string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

func gearDeviceToPeerRefreshInfo(in apitypes.DeviceInfo) apitypes.RefreshInfo {
	out := apitypes.RefreshInfo{}
	if in.Name != nil {
		out.Name = in.Name
	}
	if in.Hardware != nil {
		out.Manufacturer = in.Hardware.Manufacturer
		out.Model = in.Hardware.Model
		out.HardwareRevision = in.Hardware.HardwareRevision
	}
	return out
}

func gearToPeerGearIMEI(in apitypes.GearIMEI) apitypes.GearIMEI {
	out := apitypes.GearIMEI{
		Tac:    in.Tac,
		Serial: in.Serial,
	}
	out.Name = in.Name
	return out
}

func gearToPeerGearLabel(in apitypes.GearLabel) apitypes.GearLabel {
	return apitypes.GearLabel{
		Key:   in.Key,
		Value: in.Value,
	}
}

func gearDeviceToPeerRefreshIdentifiers(in apitypes.DeviceInfo) apitypes.RefreshIdentifiers {
	out := apitypes.RefreshIdentifiers{}
	out.Sn = in.Sn
	if in.Hardware != nil {
		if in.Hardware.Imeis != nil {
			items := make([]apitypes.GearIMEI, len(*in.Hardware.Imeis))
			for i := range *in.Hardware.Imeis {
				items[i] = gearToPeerGearIMEI((*in.Hardware.Imeis)[i])
			}
			out.Imeis = &items
		}
		if in.Hardware.Labels != nil {
			items := make([]apitypes.GearLabel, len(*in.Hardware.Labels))
			for i := range *in.Hardware.Labels {
				items[i] = gearToPeerGearLabel((*in.Hardware.Labels)[i])
			}
			out.Labels = &items
		}
	}
	return out
}

func gearDeviceToPeerRefreshVersion(in apitypes.DeviceInfo) apitypes.RefreshVersion {
	out := apitypes.RefreshVersion{}
	if in.Hardware != nil {
		out.Depot = in.Hardware.Depot
		out.FirmwareSemver = in.Hardware.FirmwareSemver
	}
	return out
}
