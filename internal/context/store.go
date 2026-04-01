package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/giztoy/giztoy-go/internal/identity"
	"github.com/giztoy/giztoy-go/internal/paths"
)

const currentLink = "current"

// Store manages the context root directory.
type Store struct {
	Root string
}

// DefaultStore returns a Store under the giztoy config directory.
func DefaultStore() (*Store, error) {
	root, err := paths.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("context: config dir: %w", err)
	}
	return &Store{Root: root}, nil
}

// Create creates a new context directory with a generated key pair and config.
func (s *Store) Create(name, serverAddr, serverPubKey string) error {
	dir := filepath.Join(s.Root, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("context: %q already exists", name)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("context: mkdir: %w", err)
	}

	if _, err := identity.LoadOrGenerate(filepath.Join(dir, "identity.key")); err != nil {
		return fmt.Errorf("context: generate key: %w", err)
	}

	cfg := Config{Server: ServerConfig{Address: serverAddr, PublicKey: serverPubKey}}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("context: marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("context: write config: %w", err)
	}

	link := filepath.Join(s.Root, currentLink)
	if _, err := os.Lstat(link); os.IsNotExist(err) {
		_ = os.Symlink(name, link)
	}

	return nil
}

// Use switches the current context by updating the symlink.
func (s *Store) Use(name string) error {
	dir := filepath.Join(s.Root, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("context: %q does not exist", name)
	}

	link := filepath.Join(s.Root, currentLink)
	_ = os.Remove(link)
	if err := os.Symlink(name, link); err != nil {
		return fmt.Errorf("context: symlink: %w", err)
	}
	return nil
}

// Current returns the currently active context, or nil if none is set.
func (s *Store) Current() (*Context, error) {
	link := filepath.Join(s.Root, currentLink)
	target, err := os.Readlink(link)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("context: readlink: %w", err)
	}

	dir := target
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(s.Root, dir)
	}
	return Load(dir)
}

// LoadByName loads a context by its plain name (no path separators allowed).
func (s *Store) LoadByName(name string) (*Context, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return nil, fmt.Errorf("context: invalid name %q", name)
	}
	dir := filepath.Join(s.Root, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("context: %q does not exist", name)
	}
	return Load(dir)
}

// List returns the names of all contexts, sorted alphabetically.
// The returned current name is empty if no current is set.
func (s *Store) List() (names []string, current string, err error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("context: readdir: %w", err)
	}

	link := filepath.Join(s.Root, currentLink)
	if target, err := os.Readlink(link); err == nil {
		current = target
	}

	for _, e := range entries {
		if e.Name() == currentLink {
			continue
		}
		if !e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, current, nil
}
