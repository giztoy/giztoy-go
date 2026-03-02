package socketopt

import (
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"
)

func TestConfigAndHelpers(t *testing.T) {
	d := DefaultConfig()
	if d.RecvBufSize != DefaultRecvBufSize {
		t.Fatalf("DefaultConfig recv=%d, want %d", d.RecvBufSize, DefaultRecvBufSize)
	}
	if d.SendBufSize != DefaultSendBufSize {
		t.Fatalf("DefaultConfig send=%d, want %d", d.SendBufSize, DefaultSendBufSize)
	}

	f := FullConfig()
	if f.BusyPollUS != DefaultBusyPollUS || !f.GRO || !f.GSO {
		t.Fatalf("FullConfig unexpected: %+v", f)
	}

	e1 := errors.New("e1")
	e2 := errors.New("e2")
	if got := firstError(nil, e1, e2); !errors.Is(got, e1) {
		t.Fatalf("firstError=%v, want e1", got)
	}
	if got := firstError(nil, nil); got != nil {
		t.Fatalf("firstError nil case=%v, want nil", got)
	}

	if got := itoa(0); got != "0" {
		t.Fatalf("itoa(0)=%s, want 0", got)
	}
	if got := itoa(12345); got != "12345" {
		t.Fatalf("itoa(12345)=%s, want 12345", got)
	}
	if got := itoa(-9); got != "-9" {
		t.Fatalf("itoa(-9)=%s, want -9", got)
	}
}

func TestApplyAndReport(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer conn.Close()

	report := Apply(conn, FullConfig())
	if report == nil {
		t.Fatal("Apply returned nil report")
	}
	if len(report.Entries) == 0 {
		t.Fatal("Apply report should contain entries")
	}

	text := report.String()
	if !strings.Contains(text, "socket optimizations") {
		t.Fatalf("report string missing header: %s", text)
	}

	if runtime.GOOS != "windows" {
		if got := GetSocketBufSize(conn, true); got <= 0 {
			t.Fatalf("GetSocketBufSize(recv)=%d, want > 0", got)
		}
		if got := GetSocketBufSize(conn, false); got <= 0 {
			t.Fatalf("GetSocketBufSize(send)=%d, want > 0", got)
		}
	}
}

func TestListenUDPReusePortAndSetReusePort(t *testing.T) {
	conn, err := ListenUDPReusePort("127.0.0.1:0")
	if err != nil {
		t.Skipf("ListenUDPReusePort unavailable on current platform: %v", err)
	}
	defer conn.Close()

	if conn.LocalAddr() == nil {
		t.Fatal("reuseport listener local addr is nil")
	}

	if err := SetReusePort(^uintptr(0)); err == nil {
		// 某些平台可能对无效 fd 也不立即报错，这里仅记录。
		t.Log("SetReusePort on invalid fd returned nil")
	}
}

func TestBatchConnFallbackOnNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-linux fallback only")
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer conn.Close()

	if bc := NewBatchConn(conn, 8); bc != nil {
		t.Fatalf("NewBatchConn non-linux should be nil, got %#v", bc)
	}

	// 覆盖 fallback 方法。
	bc := &BatchConn{}
	if n, err := bc.ReadBatch(nil); n != 0 || err != nil {
		t.Fatalf("ReadBatch fallback=(%d,%v), want (0,nil)", n, err)
	}
	if got := bc.ReceivedN(0); got != 0 {
		t.Fatalf("ReceivedN fallback=%d, want 0", got)
	}
	if got := bc.ReceivedFrom(0); got != nil {
		t.Fatalf("ReceivedFrom fallback=%v, want nil", got)
	}
}
