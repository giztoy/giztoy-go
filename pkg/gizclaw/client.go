package gizclaw

import (
	"fmt"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/adminservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/gearservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/peerpublic"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/rpc"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/serverpublic"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/giznet/gizhttp"
	"net"
	"net/http"
)

// Client connects to a remote peer server.
type Client struct {
	listener *giznet.Listener
	conn     *giznet.Conn
	serverPK giznet.PublicKey
}

// Dial connects to a server and performs the Noise handshake.
func Dial(localKey *giznet.KeyPair, serverAddr string, serverPK giznet.PublicKey) (*Client, error) {
	c := &Client{serverPK: serverPK}

	l, err := giznet.Listen(localKey,
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithServiceMuxConfig(giznet.ServiceMuxConfig{
			OnNewService: func(_ giznet.PublicKey, service uint64) bool {
				return service == ServiceServerPublic || service == ServicePeerPublic
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("gizclaw: listen: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("gizclaw: resolve addr: %w", err)
	}

	conn, err := l.Dial(serverPK, udpAddr)
	if err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("gizclaw: dial: %w", err)
	}

	c.listener = l
	c.conn = conn

	return c, nil
}

// Close releases all resources including the underlying UDP socket.
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	if c.listener != nil {
		return c.listener.Close()
	}
	return nil
}

// HTTPClient returns an HTTP client bound to a peer service.
func (c *Client) HTTPClient(service uint64) *http.Client {
	return gizhttp.NewClient(c.conn, service)
}

func (c *Client) AdminClient() (*adminservice.Client, error) {
	return adminservice.NewClient(
		"http://gizclaw",
		adminservice.WithHTTPClient(c.HTTPClient(ServiceAdmin)),
	)
}

func (c *Client) GearClient() (*gearservice.GearClient, error) {
	return gearservice.NewGearClient(
		"http://gizclaw",
		gearservice.WithHTTPClient(c.HTTPClient(ServiceGear)),
	)
}

func (c *Client) PublicClient() (*serverpublic.PublicClient, error) {
	return serverpublic.NewPublicClient(
		"http://gizclaw",
		serverpublic.WithHTTPClient(c.HTTPClient(ServiceServerPublic)),
	)
}

func (c *Client) PeriphClient() (*peerpublic.Client, error) {
	return peerpublic.NewClient(c.HTTPClient(ServicePeerPublic))
}

func (c *Client) OpenRPC() (*rpc.Client, error) {
	stream, err := c.conn.Dial(ServiceRPC)
	if err != nil {
		return nil, fmt.Errorf("gizclaw: dial rpc stream: %w", err)
	}
	return rpc.NewClient(stream), nil
}

// PeerConn returns the underlying peer connection.
func (c *Client) PeerConn() *giznet.Conn {
	return c.conn
}

// ServerPublicKey returns the expected remote server public key.
func (c *Client) ServerPublicKey() giznet.PublicKey {
	return c.serverPK
}
