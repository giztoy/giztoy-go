package client

import (
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
)

func TestProbeServerPublicReadyNilClient(t *testing.T) {
	err := probeServerPublicReady(nil)
	if err == nil {
		t.Fatal("probeServerPublicReady should fail for nil client")
	}
	if !strings.Contains(err.Error(), "nil client") {
		t.Fatalf("probeServerPublicReady error = %v", err)
	}
}

func TestProbeServerPublicReadyRequiresConnection(t *testing.T) {
	err := probeServerPublicReady(&gizclaw.Client{})
	if err == nil {
		t.Fatal("probeServerPublicReady should fail without connection")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("probeServerPublicReady error = %v", err)
	}
}
