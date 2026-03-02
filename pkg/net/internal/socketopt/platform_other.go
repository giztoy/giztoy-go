//go:build !linux

package socketopt

import "net"

func applyPlatformOptions(_ *net.UDPConn, _ Config, _ *OptimizationReport) {}

type BatchConn struct{}

func NewBatchConn(_ *net.UDPConn, _ int) *BatchConn     { return nil }
func (bc *BatchConn) ReadBatch(_ [][]byte) (int, error) { return 0, nil }
func (bc *BatchConn) ReceivedN(_ int) int               { return 0 }
func (bc *BatchConn) ReceivedFrom(_ int) *net.UDPAddr   { return nil }
