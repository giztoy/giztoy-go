package net

import (
	"sync/atomic"
	"testing"
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
