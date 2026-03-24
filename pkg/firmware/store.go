package firmware

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

type Store struct {
	root string

	mu      sync.Mutex
	depotMu map[string]*sync.Mutex
}

func NewStore(root string) *Store {
	return &Store{
		root:    root,
		depotMu: make(map[string]*sync.Mutex),
	}
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) EnsureDepot(depot string) error {
	if err := validateDepotName(depot); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(s.root, depot), 0o755)
}

func (s *Store) DepotPath(depot string) string {
	return filepath.Join(s.root, depot)
}

func (s *Store) InfoPath(depot string) string {
	return infoPath(s.root, depot)
}

func (s *Store) ChannelPath(depot string, channel Channel) string {
	return releasePath(s.root, depot, channel)
}

func (s *Store) ManifestPath(depot string, channel Channel) string {
	return manifestPath(s.root, depot, channel)
}

func (s *Store) TempPath(depot string, suffix string) string {
	return filepath.Join(s.root, depot, ".tmp-"+suffix)
}

func (s *Store) LockDepot(depot string) func() {
	if err := validateDepotName(depot); err != nil {
		return func() {}
	}
	s.mu.Lock()
	mu, ok := s.depotMu[depot]
	if !ok {
		mu = &sync.Mutex{}
		s.depotMu[depot] = mu
	}
	s.mu.Unlock()

	mu.Lock()
	return mu.Unlock
}

func (s *Store) ValidateDepot(depot string) error {
	if err := validateDepotName(depot); err != nil {
		return err
	}
	info, err := os.Stat(s.DepotPath(depot))
	if err != nil {
		if os.IsNotExist(err) {
			return ErrDepotNotFound
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("firmware: depot %s is not a directory", depot)
	}
	return nil
}

func validateDepotName(depot string) error {
	if depot == "" {
		return fmt.Errorf("firmware: empty depot name")
	}
	if strings.Contains(depot, `\`) {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	if path.IsAbs(depot) || strings.HasPrefix(depot, "/") {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	if cleaned := path.Clean(depot); cleaned != depot {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	parts := strings.Split(depot, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("firmware: invalid depot name %q", depot)
		}
	}
	return nil
}
