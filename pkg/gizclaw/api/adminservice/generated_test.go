package adminservice

import (
	"bytes"
	"testing"
)

func TestChannelRequestPathsIncludeChannelsSegment(t *testing.T) {
	t.Run("get channel", func(t *testing.T) {
		req, err := NewGetChannelRequest("http://example.com", DepotName("demo-main"), Channel("stable"))
		if err != nil {
			t.Fatalf("NewGetChannelRequest() error = %v", err)
		}
		if got := req.URL.Path; got != "/depots/demo-main/channels/stable" {
			t.Fatalf("NewGetChannelRequest() path = %q", got)
		}
	})

	t.Run("put channel", func(t *testing.T) {
		req, err := NewPutChannelRequestWithBody("http://example.com", DepotName("demo-main"), Channel("stable"), "application/octet-stream", bytes.NewReader(nil))
		if err != nil {
			t.Fatalf("NewPutChannelRequestWithBody() error = %v", err)
		}
		if got := req.URL.Path; got != "/depots/demo-main/channels/stable" {
			t.Fatalf("NewPutChannelRequestWithBody() path = %q", got)
		}
	})
}
