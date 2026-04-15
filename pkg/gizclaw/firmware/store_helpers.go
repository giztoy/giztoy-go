package firmware

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sync"

	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
)

func (s *Server) ensureDepot(depot string) error {
	if err := validateDepotName(depot); err != nil {
		return err
	}
	return s.store().MkdirAll(s.depotPath(depot))
}

func (s *Server) validateDepot(depot string) error {
	if err := validateDepotName(depot); err != nil {
		return err
	}
	info, err := s.store().Stat(s.depotPath(depot))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errDepotNotFound
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("firmware: depot %s is not a directory", depot)
	}
	return nil
}

func (s *Server) lockDepot(depot string) func() {
	if err := validateDepotName(depot); err != nil {
		return func() {}
	}
	s.mu.Lock()
	if s.depotMu == nil {
		s.depotMu = make(map[string]*sync.Mutex)
	}
	mu, ok := s.depotMu[depot]
	if !ok {
		mu = &sync.Mutex{}
		s.depotMu[depot] = mu
	}
	s.mu.Unlock()
	mu.Lock()
	return mu.Unlock
}

func (s *Server) depotPath(depot string) string {
	return depot
}

func (s *Server) infoPath(depot string) string {
	return path.Join(s.depotPath(depot), "info.json")
}

func (s *Server) channelPath(depot, channel string) string {
	return path.Join(s.depotPath(depot), channel)
}

func (s *Server) manifestPath(depot, channel string) string {
	return path.Join(s.channelPath(depot, channel), "manifest.json")
}

func (s *Server) tempPath(depot, suffix string) string {
	return path.Join(s.depotPath(depot), ".tmp-"+suffix)
}

func (s *Server) store() depotstore.Store {
	return s.Store
}
