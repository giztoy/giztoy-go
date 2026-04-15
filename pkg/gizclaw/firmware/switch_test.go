package firmware

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func TestReleaseDepot(t *testing.T) {
	t.Parallel()

	t.Run("missing depot", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.releaseDepot("missing"); !errors.Is(err, errDepotNotFound) {
			t.Fatalf("releaseDepot() error = %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		env.writeRelease("depot", Testing, "1.2.0", map[string]string{"fw.bin": "testing"})

		depot, err := env.srv.releaseDepot("depot")
		if err != nil {
			t.Fatalf("releaseDepot() unexpected error: %v", err)
		}
		if depot.Rollback.FirmwareSemver != "1.0.0" || depot.Stable.FirmwareSemver != "1.1.0" || depot.Beta.FirmwareSemver != "1.2.0" {
			t.Fatalf("releaseDepot() depot = %+v", depot)
		}
		manifest := parseManifestOrFatal(t, env.readFile("depot/stable/manifest.json"))
		if releaseChannel(manifest) != Stable {
			t.Fatalf("stable manifest channel = %q", releaseChannel(manifest))
		}
	})

	t.Run("success without previous stable", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		env.writeRelease("depot", Testing, "1.2.0", map[string]string{"fw.bin": "testing"})

		depot, err := env.srv.releaseDepot("depot")
		if err != nil {
			t.Fatalf("releaseDepot() unexpected error: %v", err)
		}
		if depot.Rollback.FirmwareSemver != "" || depot.Stable.FirmwareSemver != "1.1.0" || depot.Beta.FirmwareSemver != "1.2.0" {
			t.Fatalf("releaseDepot() depot = %+v", depot)
		}
	})

	t.Run("missing beta", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Testing, "1.2.0", map[string]string{"fw.bin": "testing"})
		if _, err := env.srv.releaseDepot("depot"); !errors.Is(err, errChannelNotFound) {
			t.Fatalf("releaseDepot() error = %v", err)
		}
	})

	t.Run("missing testing", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		if _, err := env.srv.releaseDepot("depot"); !errors.Is(err, errChannelNotFound) {
			t.Fatalf("releaseDepot() error = %v", err)
		}
	})

	t.Run("prepare switch error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		env.writeRelease("depot", Testing, "1.2.0", map[string]string{"fw.bin": "testing"})
		store := newMockStore(t)
		store.base = env.store
		store.rename = func(oldName, newName string) error { return errors.New("boom") }
		srv := &Server{Store: store}
		if _, err := srv.releaseDepot("depot"); err == nil {
			t.Fatal("releaseDepot() expected prepareSwitch error")
		}
	})

	t.Run("rewrite manifest error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		env.writeFile("depot/testing/manifest.json", `{`)
		env.writeFile("depot/testing/fw.bin", "testing")
		if _, err := env.srv.releaseDepot("depot"); err == nil {
			t.Fatal("releaseDepot() expected rewrite manifest error")
		}
	})
}

func TestRollbackDepot(t *testing.T) {
	t.Parallel()

	t.Run("missing depot", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.rollbackDepot("missing"); !errors.Is(err, errDepotNotFound) {
			t.Fatalf("rollbackDepot() error = %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.1.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Rollback, "1.0.0", map[string]string{"fw.bin": "rollback"})

		depot, err := env.srv.rollbackDepot("depot")
		if err != nil {
			t.Fatalf("rollbackDepot() unexpected error: %v", err)
		}
		if depot.Stable.FirmwareSemver != "1.0.0" || depot.Rollback.FirmwareSemver != "1.1.0" {
			t.Fatalf("rollbackDepot() depot = %+v", depot)
		}
	})

	t.Run("success without previous stable", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Rollback, "1.0.0", map[string]string{"fw.bin": "rollback"})

		depot, err := env.srv.rollbackDepot("depot")
		if err != nil {
			t.Fatalf("rollbackDepot() unexpected error: %v", err)
		}
		if depot.Stable.FirmwareSemver != "1.0.0" || depot.Rollback.FirmwareSemver != "" {
			t.Fatalf("rollbackDepot() depot = %+v", depot)
		}
	})

	t.Run("missing rollback", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.1.0", map[string]string{"fw.bin": "stable"})
		if _, err := env.srv.rollbackDepot("depot"); !errors.Is(err, errChannelNotFound) {
			t.Fatalf("rollbackDepot() error = %v", err)
		}
	})

	t.Run("prepare switch error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.1.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Rollback, "1.0.0", map[string]string{"fw.bin": "rollback"})
		store := newMockStore(t)
		store.base = env.store
		store.rename = func(oldName, newName string) error { return errors.New("boom") }
		srv := &Server{Store: store}
		if _, err := srv.rollbackDepot("depot"); err == nil {
			t.Fatal("rollbackDepot() expected prepareSwitch error")
		}
	})

	t.Run("rewrite manifest error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.1.0", map[string]string{"fw.bin": "stable"})
		env.writeFile("depot/rollback/manifest.json", `{`)
		env.writeFile("depot/rollback/fw.bin", "rollback")
		if _, err := env.srv.rollbackDepot("depot"); err == nil {
			t.Fatal("rollbackDepot() expected rewrite manifest error")
		}
	})
}

func TestPrepareSwitchAndRewriteManifest(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
	env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})

	backups, restore, err := env.srv.prepareSwitch(
		"depot",
		map[string]string{
			"stable": env.srv.channelPath("depot", "stable"),
			"beta":   env.srv.channelPath("depot", "beta"),
		},
		map[string]string{
			"stable": "beta",
			"beta":   "stable",
		},
	)
	if err != nil {
		t.Fatalf("prepareSwitch() unexpected error: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("prepareSwitch() backups = %#v", backups)
	}
	if err := restore(); err != nil {
		t.Fatalf("restore() unexpected error: %v", err)
	}

	if err := env.srv.rewriteManifestChannel("depot", Stable); err != nil {
		t.Fatalf("rewriteManifestChannel() unexpected error: %v", err)
	}
	manifest := parseManifestOrFatal(t, env.readFile("depot/stable/manifest.json"))
	if releaseChannel(manifest) != Stable {
		t.Fatalf("rewriteManifestChannel() channel = %q", releaseChannel(manifest))
	}

	if err := env.srv.rewriteManifestChannel("depot", Testing); !errors.Is(err, errChannelNotFound) {
		t.Fatalf("rewriteManifestChannel() missing error = %v", err)
	}

	env.writeFile("depot/beta/manifest.json", `{`)
	if err := env.srv.rewriteManifestChannel("depot", Beta); err == nil {
		t.Fatal("rewriteManifestChannel() expected parse error")
	}

	store := newMockStore(t)
	store.readFile = func(name string) ([]byte, error) { return nil, errors.New("boom") }
	srv := &Server{Store: store}
	if err := srv.rewriteManifestChannel("depot", Stable); err == nil {
		t.Fatal("rewriteManifestChannel() expected read error")
	}
}

func TestPrepareSwitchErrors(t *testing.T) {
	t.Parallel()

	t.Run("stat error", func(t *testing.T) {
		t.Parallel()
		store := newMockStore(t)
		store.stat = func(name string) (fs.FileInfo, error) { return nil, errors.New("boom") }
		srv := &Server{Store: store}
		if _, _, err := srv.prepareSwitch("depot", map[string]string{"stable": "depot/stable"}, map[string]string{}); err == nil {
			t.Fatal("prepareSwitch() expected stat error")
		}
	})

	t.Run("rename backup error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		store := newMockStore(t)
		store.base = env.store
		store.rename = func(oldName, newName string) error {
			return errors.New("boom")
		}
		srv := &Server{Store: store}
		if _, _, err := srv.prepareSwitch("depot", map[string]string{"stable": "depot/stable"}, map[string]string{}); err == nil {
			t.Fatal("prepareSwitch() expected rename error")
		}
	})

	t.Run("target rename error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		store := newMockStore(t)
		store.base = env.store
		baseRename := store.base.Rename
		store.rename = func(oldName, newName string) error {
			if oldName == "depot/.bak-beta" && newName == "depot/stable" {
				return errors.New("boom")
			}
			return baseRename(oldName, newName)
		}
		srv := &Server{Store: store}
		if _, _, err := srv.prepareSwitch(
			"depot",
			map[string]string{
				"stable": "depot/stable",
				"beta":   "depot/beta",
			},
			map[string]string{"stable": "beta"},
		); err == nil {
			t.Fatal("prepareSwitch() expected target rename error")
		}
	})
}

func parseManifestOrFatal(t *testing.T, data []byte) adminservice.DepotRelease {
	t.Helper()
	release, err := parseManifest(data)
	if err != nil {
		t.Fatalf("parseManifest() unexpected error: %v", err)
	}
	return release
}
