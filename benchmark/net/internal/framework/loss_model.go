package framework

import "sync/atomic"

// LossModel is a deterministic packet drop model for benchmark simulation.
//
// Drop decision uses an LCG pseudo-random sequence so results are reproducible
// when the same seed/rate is used.
type LossModel struct {
	rate    float64
	state   atomic.Uint64
	attempt atomic.Uint64
	drop    atomic.Uint64
}

// NewLossModel creates a new loss model.
// rate is in [0,1], where 0 means no drop and 1 means always drop.
func NewLossModel(rate float64, seed uint64) *LossModel {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	m := &LossModel{rate: rate}
	m.state.Store(seed)
	return m
}

// ShouldDrop returns true when current packet should be dropped.
func (m *LossModel) ShouldDrop() bool {
	m.attempt.Add(1)
	if m.rate <= 0 {
		return false
	}
	if m.rate >= 1 {
		m.drop.Add(1)
		return true
	}

	// 64-bit LCG constants (Numerical Recipes variant).
	x := m.state.Add(6364136223846793005) + 1442695040888963407
	r := float64(x%1_000_000) / 1_000_000.0
	if r < m.rate {
		m.drop.Add(1)
		return true
	}
	return false
}

// Attempt returns number of packets tested by the model.
func (m *LossModel) Attempt() uint64 {
	return m.attempt.Load()
}

// Dropped returns number of packets dropped by the model.
func (m *LossModel) Dropped() uint64 {
	return m.drop.Load()
}

// Rate returns configured drop rate in [0,1].
func (m *LossModel) Rate() float64 {
	return m.rate
}
