package net

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestNewKCP_CreateFailureSafeReturn(t *testing.T) {
	oldCreate := createKCP
	oldSetOutput := setKCPOutput
	t.Cleanup(func() {
		createKCP = oldCreate
		setKCPOutput = oldSetOutput
	})

	var setOutputCalled atomic.Bool
	createKCP = func(conv uint32, user unsafe.Pointer) unsafe.Pointer {
		_ = conv
		_ = user
		return nil
	}
	setKCPOutput = func(kcp unsafe.Pointer) {
		_ = kcp
		setOutputCalled.Store(true)
	}

	k := NewKCP(7, func([]byte) {})
	if k == nil {
		t.Fatal("NewKCP returned nil KCP wrapper")
	}
	if k.kcp != nil {
		t.Fatal("NewKCP should keep k.kcp=nil when create fails")
	}
	if setOutputCalled.Load() {
		t.Fatal("setKCPOutput should not be called when create fails")
	}

	kcpRegistryMu.RLock()
	_, exists := kcpRegistry[k.id]
	kcpRegistryMu.RUnlock()
	if exists {
		t.Fatal("registry should not keep failed KCP entry")
	}

	// 失败对象应可安全调用方法，不发生崩溃。
	k.SetOutput(nil)
	if n := k.Send([]byte("x")); n != -1 {
		t.Fatalf("Send on failed KCP = %d, want -1", n)
	}
	k.Release()
}

func drainPackets(mu *sync.Mutex, packets *[][]byte) [][]byte {
	mu.Lock()
	defer mu.Unlock()
	out := append([][]byte(nil), (*packets)...)
	*packets = nil
	return out
}

func TestKCPNilGuardsAndHelpers(t *testing.T) {
	k := &KCP{conv: 42}

	k.SetOutput(func([]byte) {})
	if h := k.outputPtr.Load(); h == nil || h.fn == nil {
		t.Fatal("SetOutput should store non-nil handler")
	}
	k.SetOutput(nil)
	if h := k.outputPtr.Load(); h != nil {
		t.Fatal("SetOutput(nil) should clear handler")
	}

	if got := k.Conv(); got != 42 {
		t.Fatalf("Conv() = %d, want 42", got)
	}

	if n := k.Send([]byte("x")); n != -1 {
		t.Fatalf("Send(nil-kcp) = %d, want -1", n)
	}
	buf := make([]byte, 16)
	if n := k.Recv(buf); n != -1 {
		t.Fatalf("Recv(nil-kcp) = %d, want -1", n)
	}
	if n := k.Input([]byte("x")); n != -1 {
		t.Fatalf("Input(nil-kcp) = %d, want -1", n)
	}

	k.Update(123)
	if next := k.Check(123); next != 123 {
		t.Fatalf("Check(nil-kcp) = %d, want 123", next)
	}
	k.Flush()

	if got := k.PeekSize(); got != -1 {
		t.Fatalf("PeekSize(nil-kcp) = %d, want -1", got)
	}
	if got := k.SetMTU(1400); got != -1 {
		t.Fatalf("SetMTU(nil-kcp) = %d, want -1", got)
	}
	if got := k.SetWndSize(64, 64); got != -1 {
		t.Fatalf("SetWndSize(nil-kcp) = %d, want -1", got)
	}
	if got := k.SetNodelay(1, 10, 2, 1); got != -1 {
		t.Fatalf("SetNodelay(nil-kcp) = %d, want -1", got)
	}
	if got := k.State(); got != -1 {
		t.Fatalf("State(nil-kcp) = %d, want -1", got)
	}
	if got := k.WaitSnd(); got != 0 {
		t.Fatalf("WaitSnd(nil-kcp) = %d, want 0", got)
	}

	if conv := GetConv([]byte{1, 2, 3}); conv != 0 {
		t.Fatalf("GetConv(short) = %d, want 0", conv)
	}

	// nil kcp 场景下 DefaultConfig 也应安全无副作用。
	k.DefaultConfig()
}

func TestKCPRoundTripAndCoreAPIs(t *testing.T) {
	var (
		outA [][]byte
		outB [][]byte
		muA  sync.Mutex
		muB  sync.Mutex
	)

	const convID uint32 = 1001

	kcpA := NewKCP(convID, func(data []byte) {
		muA.Lock()
		outA = append(outA, append([]byte(nil), data...))
		muA.Unlock()
	})
	if kcpA == nil || kcpA.kcp == nil {
		t.Fatal("NewKCP(A) returned nil control block")
	}
	defer kcpA.Release()

	kcpB := NewKCP(convID, func(data []byte) {
		muB.Lock()
		outB = append(outB, append([]byte(nil), data...))
		muB.Unlock()
	})
	if kcpB == nil || kcpB.kcp == nil {
		t.Fatal("NewKCP(B) returned nil control block")
	}
	defer kcpB.Release()

	if ret := kcpA.SetMTU(1200); ret < 0 {
		t.Fatalf("SetMTU() failed with %d", ret)
	}
	if ret := kcpA.SetWndSize(128, 128); ret < 0 {
		t.Fatalf("SetWndSize() failed with %d", ret)
	}
	if ret := kcpA.SetNodelay(1, 10, 2, 1); ret < 0 {
		t.Fatalf("SetNodelay() failed with %d", ret)
	}
	kcpB.DefaultConfig()

	if got := kcpA.Conv(); got != convID {
		t.Fatalf("Conv() = %d, want %d", got, convID)
	}
	if got := kcpA.State(); got < 0 {
		t.Fatalf("State() = %d, want >= 0", got)
	}
	if got := kcpA.WaitSnd(); got < 0 {
		t.Fatalf("WaitSnd() before send = %d, want >= 0", got)
	}

	payload := []byte("hello third-party kcp")
	if n := kcpA.Send(payload); n < 0 {
		t.Fatalf("Send() failed with %d", n)
	}
	if got := kcpA.WaitSnd(); got < 0 {
		t.Fatalf("WaitSnd() after send = %d, want >= 0", got)
	}

	received := make([]byte, 0, len(payload))
	buf := make([]byte, 4096)
	convChecked := false

	for i := 0; i < 200 && len(received) == 0; i++ {
		now := uint32(time.Now().UnixMilli())
		kcpA.Update(now)
		kcpA.Flush()
		_ = kcpA.Check(now)

		pktsA := drainPackets(&muA, &outA)
		for _, pkt := range pktsA {
			if !convChecked {
				if got := GetConv(pkt); got != convID {
					t.Fatalf("GetConv(packet) = %d, want %d", got, convID)
				}
				convChecked = true
			}
			if ret := kcpB.Input(pkt); ret < 0 {
				t.Fatalf("Input() failed with %d", ret)
			}
		}

		now = uint32(time.Now().UnixMilli())
		kcpB.Update(now)
		kcpB.Flush()

		pktsB := drainPackets(&muB, &outB)
		for _, pkt := range pktsB {
			if ret := kcpA.Input(pkt); ret < 0 {
				t.Fatalf("Input(ack) failed with %d", ret)
			}
		}

		if kcpB.PeekSize() > 0 {
			n := kcpB.Recv(buf)
			if n < 0 {
				t.Fatalf("Recv() failed with %d", n)
			}
			received = append(received, buf[:n]...)
		}

		time.Sleep(2 * time.Millisecond)
	}

	if !convChecked {
		t.Fatal("expected at least one packet to validate conv")
	}
	if !bytes.Equal(received, payload) {
		t.Fatalf("received %q, want %q", received, payload)
	}
}

func TestKCPOutputCallbackBranches(t *testing.T) {
	var called atomic.Int32

	k := NewKCP(2002, func([]byte) {
		called.Add(1)
	})
	if k == nil || k.kcp == nil {
		t.Fatal("NewKCP returned nil control block")
	}
	defer k.Release()

	// 分支1：output handler 为空。
	k.SetOutput(nil)
	if n := k.Send([]byte("no-callback")); n < 0 {
		t.Fatalf("Send() failed with %d", n)
	}
	k.Update(uint32(time.Now().UnixMilli()))
	k.Flush()
	if got := called.Load(); got != 0 {
		t.Fatalf("output callback called %d times, want 0", got)
	}

	// 分支2：registry miss。
	k.SetOutput(func([]byte) {
		called.Add(1)
	})
	kcpRegistryMu.Lock()
	delete(kcpRegistry, k.id)
	kcpRegistryMu.Unlock()

	if n := k.Send([]byte("registry-miss")); n < 0 {
		t.Fatalf("Send() failed with %d", n)
	}
	k.Update(uint32(time.Now().UnixMilli()))
	k.Flush()
	if got := called.Load(); got != 0 {
		t.Fatalf("output callback called %d times, want 0 under registry miss", got)
	}
}
