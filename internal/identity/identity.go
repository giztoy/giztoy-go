package identity

import (
	"fmt"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"os"
	"path/filepath"
)

// LoadOrGenerate loads a key pair from path, or generates and saves a new one
// if the file does not exist. The file contains the raw 32-byte private key.
func LoadOrGenerate(path string) (*giznet.KeyPair, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return parseKeyFile(data)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("identity: read %s: %w", path, err)
	}
	return generate(path)
}

// Load loads a key pair from an existing file.
func Load(path string) (*giznet.KeyPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("identity: read %s: %w", path, err)
	}
	return parseKeyFile(data)
}

func parseKeyFile(data []byte) (*giznet.KeyPair, error) {
	if len(data) != giznet.KeySize {
		return nil, fmt.Errorf("identity: invalid key file: got %d bytes, want %d", len(data), giznet.KeySize)
	}
	var key giznet.Key
	copy(key[:], data)
	return giznet.NewKeyPair(key)
}

func generate(path string) (*giznet.KeyPair, error) {
	kp, err := giznet.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("identity: generate: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("identity: mkdir: %w", err)
	}
	if err := os.WriteFile(path, kp.Private[:], 0o600); err != nil {
		return nil, fmt.Errorf("identity: write %s: %w", path, err)
	}
	return kp, nil
}
