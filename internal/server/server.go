package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/giztoy/giztoy-go/internal/identity"
	"github.com/giztoy/giztoy-go/internal/paths"
	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/gizclaw"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/adminservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/gearservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/rpc"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/serverpublic"
)

var BuildCommit = "dev"

// Server is the assembled application server.
type Server = gizclaw.Server

// Config holds server startup parameters.
type Config struct {
	DataDir    string
	ListenAddr string
	ConfigPath string
	Stores     map[string]stores.Config
	Gears      GearsConfig
	Depots     DepotsConfig
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	cfgDir, _ := paths.ConfigDir()
	return Config{
		DataDir:    filepath.Join(cfgDir, "server"),
		ListenAddr: ":9820",
	}
}

// New loads config, wires dependencies, and returns a gizclaw.Server.
func New(cfg Config) (*Server, error) {
	if cfg.ConfigPath != "" {
		fileCfg, err := LoadConfig(cfg.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("server: load config: %w", err)
		}
		cfg = mergeFileConfig(cfg, fileCfg)
	}
	defaults := DefaultConfig()
	if cfg.DataDir == "" {
		cfg.DataDir = defaults.DataDir
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaults.ListenAddr
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	keyPath := filepath.Join(cfg.DataDir, "identity.key")
	kp, err := identity.LoadOrGenerate(keyPath)
	if err != nil {
		return nil, fmt.Errorf("server: identity: %w", err)
	}

	ss, err := stores.New(cfg.DataDir, cfg.Stores)
	if err != nil {
		return nil, fmt.Errorf("server: stores: %w", err)
	}
	closeStores := func() error { return ss.Close() }

	gearsKV, err := ss.KV(cfg.Gears.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: gears store: %w", err)
	}
	gearStore := gears.NewStore(gearsKV)
	gearService := gears.NewService(gearStore, cfg.Gears.RegistrationTokens)

	fwStore, err := ss.FS(cfg.Depots.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: firmware store: %w", err)
	}
	if err := os.MkdirAll(fwStore.Root(), 0o755); err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: firmware dir: %w", err)
	}
	fwScanner := firmware.NewScanner(fwStore)
	fwUploader := firmware.NewUploader(fwStore, fwScanner)
	fwSwitcher := firmware.NewSwitcher(fwStore, fwScanner)
	fwOTA := firmware.NewOTAService(fwStore, fwScanner)

	manager := gizclaw.NewManager(gearService)
	peerServer := &gizclaw.PeerServer{
		Manager: manager,
		Admin: &adminservice.Server{
			FirmwareScanner:  fwScanner,
			FirmwareUploader: fwUploader,
			FirmwareSwitcher: fwSwitcher,
		},
		Gear: &gearservice.GearServer{
			Gears:       gearService,
			FirmwareOTA: fwOTA,
			Manager:     manager,
		},
		Public: &serverpublic.PublicServer{
			BuildCommit:     BuildCommit,
			ServerPublicKey: kp.Public.String(),
			Gears:           gearService,
			FirmwareOTA:     fwOTA,
			PeerServer:      manager,
		},
		RPC: rpc.NewServer(&rpc.RPCServer{}),
	}

	return &gizclaw.Server{
		KeyPair:        kp,
		Manager:        manager,
		PeerServer:     peerServer,
		SecurityPolicy: gizclaw.GearsSecurityPolicy{Gears: gearService},
		Cleanup:        closeStores,
	}, nil
}
