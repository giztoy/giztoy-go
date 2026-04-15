package giznet

import "github.com/GizClaw/gizclaw-go/pkg/giznet/internal/noise"

const KeySize = noise.KeySize

type Key = noise.Key
type KeyPair = noise.KeyPair
type PublicKey = noise.PublicKey

var GenerateKeyPair = noise.GenerateKeyPair
var NewKeyPair = noise.NewKeyPair
var KeyFromHex = noise.KeyFromHex
