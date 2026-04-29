package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/pion/webrtc/v4"
)

func TestPlayWebRTCOfferRouteRejectsInvalidRequests(t *testing.T) {
	mux := http.NewServeMux()
	registerPlayUIRoutes(mux, nil)

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/webrtc/offer", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webrtc/offer", strings.NewReader("{"))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("not offer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webrtc/offer", strings.NewReader(`{"type":"answer","sdp":"v=0"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webrtc/offer", strings.NewReader(`{"type":"offer","sdp":"v=0"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestPlayWebRTCOfferRouteAcceptsMediaAndDataChannelOffer(t *testing.T) {
	offerPC, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection offer: %v", err)
	}
	defer func() { _ = offerPC.Close() }()

	if _, err := offerPC.CreateDataChannel("rpc", nil); err != nil {
		t.Fatalf("CreateDataChannel: %v", err)
	}
	if _, err := offerPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		t.Fatalf("AddTransceiverFromKind(audio): %v", err)
	}
	if _, err := offerPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatalf("AddTransceiverFromKind(video): %v", err)
	}

	offer, err := offerPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(offerPC)
	if err := offerPC.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription(offer): %v", err)
	}
	<-gatherComplete

	body, err := json.Marshal(playWebRTCOfferRequest{
		SDP:  offerPC.LocalDescription().SDP,
		Type: offerPC.LocalDescription().Type.String(),
	})
	if err != nil {
		t.Fatalf("Marshal offer: %v", err)
	}

	mux := http.NewServeMux()
	registerPlayUIRoutes(mux, &gizclaw.Client{})
	req := httptest.NewRequest(http.MethodPost, "/webrtc/offer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, strings.TrimSpace(rec.Body.String()))
	}

	var answer playWebRTCAnswerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("Unmarshal answer: %v; body=%q", err, rec.Body.String())
	}
	if answer.Type != webrtc.SDPTypeAnswer.String() || strings.TrimSpace(answer.SDP) == "" {
		t.Fatalf("answer = %+v, want non-empty answer SDP", answer)
	}
}

func TestPlayWebRTCDataChannelReturnsRPCErrorWhenClientDisconnected(t *testing.T) {
	offerPC, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection offer: %v", err)
	}
	defer func() { _ = offerPC.Close() }()

	if _, err := offerPC.CreateDataChannel("rpc-bootstrap", nil); err != nil {
		t.Fatalf("CreateDataChannel(bootstrap): %v", err)
	}

	offer, err := offerPC.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(offerPC)
	if err := offerPC.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription(offer): %v", err)
	}
	<-gatherComplete

	body, err := json.Marshal(playWebRTCOfferRequest{
		SDP:  offerPC.LocalDescription().SDP,
		Type: offerPC.LocalDescription().Type.String(),
	})
	if err != nil {
		t.Fatalf("Marshal offer: %v", err)
	}

	mux := http.NewServeMux()
	registerPlayUIRoutes(mux, &gizclaw.Client{})
	req := httptest.NewRequest(http.MethodPost, "/webrtc/offer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, strings.TrimSpace(rec.Body.String()))
	}

	var answer playWebRTCAnswerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("Unmarshal answer: %v; body=%q", err, rec.Body.String())
	}
	if err := offerPC.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer.SDP,
	}); err != nil {
		t.Fatalf("SetRemoteDescription(answer): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for offerPC.ConnectionState() != webrtc.PeerConnectionStateConnected {
		select {
		case <-ctx.Done():
			t.Fatalf("peer connection did not connect: %v", offerPC.ConnectionState())
		case <-time.After(10 * time.Millisecond):
		}
	}

	responseCh := make(chan string, 1)
	rpcChannel, err := offerPC.CreateDataChannel("rpc:test", nil)
	if err != nil {
		t.Fatalf("CreateDataChannel(rpc): %v", err)
	}
	rpcChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		responseCh <- string(msg.Data)
	})
	rpcChannel.OnOpen(func() {
		if err := rpcChannel.SendText(`{"v":1,"id":"test","method":"peer.ping","params":{"clientSendTime":1}}`); err != nil {
			t.Errorf("SendText: %v", err)
		}
	})

	select {
	case <-ctx.Done():
		t.Fatal("timed out waiting for rpc data channel response")
	case response := <-responseCh:
		if !strings.Contains(response, `"id":"test"`) {
			t.Fatalf("response = %q, want id", response)
		}
		if !strings.Contains(response, `"error"`) || !strings.Contains(response, "client is not connected") {
			t.Fatalf("response = %q, want disconnected client error", response)
		}
	}
}
