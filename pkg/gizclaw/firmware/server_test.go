package firmware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http/httptest"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/gofiber/fiber/v2"
)

func TestListDepotsHandler(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeDepotInfo("depot", "fw.bin")

		resp, err := env.srv.ListDepots(context.Background(), adminservice.ListDepotsRequestObject{})
		if err != nil {
			t.Fatalf("ListDepots() unexpected error: %v", err)
		}
		okResp, ok := resp.(adminservice.ListDepots200JSONResponse)
		if !ok || len(okResp.Items) != 1 || okResp.Items[0].Name != "depot" {
			t.Fatalf("ListDepots() response = %#v", resp)
		}
	})

	t.Run("store error", func(t *testing.T) {
		t.Parallel()
		store := newMockStore(t)
		store.walkDir = func(root string, fn fs.WalkDirFunc) error { return errors.New("boom") }
		srv := &Server{Store: store}

		resp, err := srv.ListDepots(context.Background(), adminservice.ListDepotsRequestObject{})
		if err != nil {
			t.Fatalf("ListDepots() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.ListDepots500JSONResponse); !ok {
			t.Fatalf("ListDepots() response = %#v", resp)
		}
	})

	t.Run("scan depot error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeFile("depot/stable/manifest.json", `{`)
		resp, err := env.srv.ListDepots(context.Background(), adminservice.ListDepotsRequestObject{})
		if err != nil {
			t.Fatalf("ListDepots() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.ListDepots500JSONResponse); !ok {
			t.Fatalf("ListDepots() response = %#v", resp)
		}
	})
}

func TestAdminHandlers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("get depot invalid path", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.GetDepot(ctx, adminservice.GetDepotRequestObject{Depot: "%"}); err == nil {
			t.Fatal("GetDepot() expected invalid params error")
		}
	})

	t.Run("get depot not found", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		resp, err := env.srv.GetDepot(ctx, adminservice.GetDepotRequestObject{Depot: "missing"})
		if err != nil {
			t.Fatalf("GetDepot() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.GetDepot404JSONResponse); !ok {
			t.Fatalf("GetDepot() response = %#v", resp)
		}
	})

	t.Run("get depot success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeDepotInfo("depot", "fw.bin")
		resp, err := env.srv.GetDepot(ctx, adminservice.GetDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("GetDepot() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.GetDepot200JSONResponse); !ok {
			t.Fatalf("GetDepot() response = %#v", resp)
		}
	})

	t.Run("put depot info validation", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)

		resp, err := env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{})
		if err != nil {
			t.Fatalf("PutDepotInfo() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutDepotInfo400JSONResponse); !ok {
			t.Fatalf("PutDepotInfo() response = %#v", resp)
		}

		info := depotInfo("../bad")
		resp, err = env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "depot", Body: &info})
		if err != nil {
			t.Fatalf("PutDepotInfo() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutDepotInfo400JSONResponse); !ok {
			t.Fatalf("PutDepotInfo() response = %#v", resp)
		}
	})

	t.Run("put depot info invalid path and invalid depot name", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		info := depotInfo("fw.bin")
		if _, err := env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "%", Body: &info}); err == nil {
			t.Fatal("PutDepotInfo() expected invalid params error")
		}
		if _, err := env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "%2Fbad", Body: &info}); err == nil {
			t.Fatal("PutDepotInfo() expected invalid depot name error")
		}
	})

	t.Run("put depot info internal error", func(t *testing.T) {
		t.Parallel()
		store := newMockStore(t)
		base := store.base
		store.readFile = func(name string) ([]byte, error) {
			if name == "depot/info.json" {
				return nil, errors.New("boom")
			}
			return base.ReadFile(name)
		}
		srv := &Server{Store: store}
		info := depotInfo("fw.bin")
		resp, err := srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "depot", Body: &info})
		if err != nil {
			t.Fatalf("PutDepotInfo() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutDepotInfo500JSONResponse); !ok {
			t.Fatalf("PutDepotInfo() response = %#v", resp)
		}
	})

	t.Run("put depot info success and conflict", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		info := depotInfo("fw.bin")
		resp, err := env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "depot", Body: &info})
		if err != nil {
			t.Fatalf("PutDepotInfo() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutDepotInfo200JSONResponse); !ok {
			t.Fatalf("PutDepotInfo() response = %#v", resp)
		}

		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "firmware"})
		other := depotInfo("other.bin")
		resp, err = env.srv.PutDepotInfo(ctx, adminservice.PutDepotInfoRequestObject{Depot: "depot", Body: &other})
		if err != nil {
			t.Fatalf("PutDepotInfo() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutDepotInfo409JSONResponse); !ok {
			t.Fatalf("PutDepotInfo() response = %#v", resp)
		}
	})

	t.Run("get channel responses", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "firmware"})

		resp, err := env.srv.GetChannel(ctx, adminservice.GetChannelRequestObject{Depot: "depot", Channel: adminservice.Channel(Stable)})
		if err != nil {
			t.Fatalf("GetChannel() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.GetChannel200JSONResponse); !ok {
			t.Fatalf("GetChannel() response = %#v", resp)
		}

		resp, err = env.srv.GetChannel(ctx, adminservice.GetChannelRequestObject{Depot: "depot", Channel: adminservice.Channel(Beta)})
		if err != nil {
			t.Fatalf("GetChannel() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.GetChannel404JSONResponse); !ok {
			t.Fatalf("GetChannel() response = %#v", resp)
		}
	})

	t.Run("get channel invalid path and missing depot", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.GetChannel(ctx, adminservice.GetChannelRequestObject{Depot: "%", Channel: adminservice.Channel(Beta)}); err == nil {
			t.Fatal("GetChannel() expected invalid params error")
		}
		resp, err := env.srv.GetChannel(ctx, adminservice.GetChannelRequestObject{Depot: "missing", Channel: adminservice.Channel(Beta)})
		if err != nil {
			t.Fatalf("GetChannel() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.GetChannel404JSONResponse); !ok {
			t.Fatalf("GetChannel() response = %#v", resp)
		}
	})

	t.Run("put channel conflict", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		resp, err := env.srv.PutChannel(ctx, adminservice.PutChannelRequestObject{
			Depot:   "depot",
			Channel: adminservice.Channel(Beta),
			Body:    bytes.NewReader([]byte("not-a-tar")),
		})
		if err != nil {
			t.Fatalf("PutChannel() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutChannel409JSONResponse); !ok {
			t.Fatalf("PutChannel() response = %#v", resp)
		}
	})

	t.Run("put channel success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		data := buildTar(t,
			tarEntry{Name: "manifest.json", Data: mustJSON(t, depotReleaseForFiles(Beta, "1.0.0", map[string]string{"fw.bin": "firmware"}))},
			tarEntry{Name: "fw.bin", Data: []byte("firmware")},
		)
		resp, err := env.srv.PutChannel(ctx, adminservice.PutChannelRequestObject{
			Depot:   "depot",
			Channel: adminservice.Channel(Beta),
			Body:    bytes.NewReader(data),
		})
		if err != nil {
			t.Fatalf("PutChannel() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.PutChannel200JSONResponse); !ok {
			t.Fatalf("PutChannel() response = %#v", resp)
		}
	})

	t.Run("put channel invalid path", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.PutChannel(ctx, adminservice.PutChannelRequestObject{
			Depot:   "%",
			Channel: adminservice.Channel(Beta),
			Body:    bytes.NewReader(nil),
		}); err == nil {
			t.Fatal("PutChannel() expected invalid params error")
		}
	})

	t.Run("release and rollback response mapping", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)

		releaseResp, err := env.srv.ReleaseDepot(ctx, adminservice.ReleaseDepotRequestObject{Depot: "missing"})
		if err != nil {
			t.Fatalf("ReleaseDepot() unexpected error: %v", err)
		}
		if _, ok := releaseResp.(adminservice.ReleaseDepot404JSONResponse); !ok {
			t.Fatalf("ReleaseDepot() response = %#v", releaseResp)
		}

		env.writeDepotInfo("depot", "fw.bin")
		rollbackResp, err := env.srv.RollbackDepot(ctx, adminservice.RollbackDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("RollbackDepot() unexpected error: %v", err)
		}
		if _, ok := rollbackResp.(adminservice.RollbackDepot409JSONResponse); !ok {
			t.Fatalf("RollbackDepot() response = %#v", rollbackResp)
		}

		releaseConflictEnv := newTestEnv(t)
		releaseConflictEnv.writeDepotInfo("depot", "fw.bin")
		releaseResp, err = releaseConflictEnv.srv.ReleaseDepot(ctx, adminservice.ReleaseDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("ReleaseDepot() unexpected error: %v", err)
		}
		if _, ok := releaseResp.(adminservice.ReleaseDepot409JSONResponse); !ok {
			t.Fatalf("ReleaseDepot() response = %#v", releaseResp)
		}
	})

	t.Run("release and rollback success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "stable"})
		env.writeRelease("depot", Beta, "1.1.0", map[string]string{"fw.bin": "beta"})
		env.writeRelease("depot", Testing, "1.2.0", map[string]string{"fw.bin": "testing"})

		releaseResp, err := env.srv.ReleaseDepot(ctx, adminservice.ReleaseDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("ReleaseDepot() unexpected error: %v", err)
		}
		if _, ok := releaseResp.(adminservice.ReleaseDepot200JSONResponse); !ok {
			t.Fatalf("ReleaseDepot() response = %#v", releaseResp)
		}

		rollbackResp, err := env.srv.RollbackDepot(ctx, adminservice.RollbackDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("RollbackDepot() unexpected error: %v", err)
		}
		if _, ok := rollbackResp.(adminservice.RollbackDepot200JSONResponse); !ok {
			t.Fatalf("RollbackDepot() response = %#v", rollbackResp)
		}
	})

	t.Run("release internal error and rollback invalid path", func(t *testing.T) {
		t.Parallel()
		store := newMockStore(t)
		store.stat = func(name string) (fs.FileInfo, error) { return nil, errors.New("boom") }
		srv := &Server{Store: store}

		releaseResp, err := srv.ReleaseDepot(ctx, adminservice.ReleaseDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("ReleaseDepot() unexpected error: %v", err)
		}
		if _, ok := releaseResp.(adminservice.ReleaseDepot500JSONResponse); !ok {
			t.Fatalf("ReleaseDepot() response = %#v", releaseResp)
		}

		if _, err := srv.RollbackDepot(ctx, adminservice.RollbackDepotRequestObject{Depot: "%"}); err == nil {
			t.Fatal("RollbackDepot() expected invalid params error")
		}
	})

	t.Run("release invalid path and rollback missing depot", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, err := env.srv.ReleaseDepot(ctx, adminservice.ReleaseDepotRequestObject{Depot: "%"}); err == nil {
			t.Fatal("ReleaseDepot() expected invalid params error")
		}
		resp, err := env.srv.RollbackDepot(ctx, adminservice.RollbackDepotRequestObject{Depot: "missing"})
		if err != nil {
			t.Fatalf("RollbackDepot() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.RollbackDepot404JSONResponse); !ok {
			t.Fatalf("RollbackDepot() response = %#v", resp)
		}
	})

	t.Run("rollback internal error", func(t *testing.T) {
		t.Parallel()
		store := newMockStore(t)
		store.stat = func(name string) (fs.FileInfo, error) { return nil, errors.New("boom") }
		srv := &Server{Store: store}
		resp, err := srv.RollbackDepot(ctx, adminservice.RollbackDepotRequestObject{Depot: "depot"})
		if err != nil {
			t.Fatalf("RollbackDepot() unexpected error: %v", err)
		}
		if _, ok := resp.(adminservice.RollbackDepot500JSONResponse); !ok {
			t.Fatalf("RollbackDepot() response = %#v", resp)
		}
	})
}

func TestGetGearOTAHandler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("resolver missing", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		resp, err := env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{})
		if err != nil {
			t.Fatalf("GetGearOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(gearservice.GetGearOTA404JSONResponse); !ok {
			t.Fatalf("GetGearOTA() response = %#v", resp)
		}
	})

	t.Run("invalid public key", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		if _, err := env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{PublicKey: "%"}); err == nil {
			t.Fatal("GetGearOTA() expected invalid params error")
		}
	})

	t.Run("resolver says unavailable", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "", "", nil
		}
		resp, err := env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{PublicKey: "peer"})
		if err != nil {
			t.Fatalf("GetGearOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(gearservice.GetGearOTA404JSONResponse); !ok {
			t.Fatalf("GetGearOTA() response = %#v", resp)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.0.0", map[string]string{"fw.bin": "firmware"})
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		resp, err := env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{PublicKey: "peer"})
		if err != nil {
			t.Fatalf("GetGearOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(gearservice.GetGearOTA200JSONResponse); !ok {
			t.Fatalf("GetGearOTA() response = %#v", resp)
		}
	})

	t.Run("resolver error and firmware missing", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "", "", errors.New("boom")
		}
		resp, err := env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{PublicKey: "peer"})
		if err != nil {
			t.Fatalf("GetGearOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(gearservice.GetGearOTA404JSONResponse); !ok {
			t.Fatalf("GetGearOTA() response = %#v", resp)
		}

		env = newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Beta, nil
		}
		resp, err = env.srv.GetGearOTA(ctx, gearservice.GetGearOTARequestObject{PublicKey: "peer"})
		if err != nil {
			t.Fatalf("GetGearOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(gearservice.GetGearOTA404JSONResponse); !ok {
			t.Fatalf("GetGearOTA() response = %#v", resp)
		}
	})
}

func TestServerPublicGetOTA(t *testing.T) {
	t.Parallel()

	t.Run("resolver missing", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		resp, err := env.srv.GetOTA(context.Background(), serverpublic.GetOTARequestObject{})
		if err != nil {
			t.Fatalf("GetOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.GetOTA404JSONResponse); !ok {
			t.Fatalf("GetOTA() response = %#v", resp)
		}
	})

	t.Run("caller missing", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		resp, err := env.srv.GetOTA(context.Background(), serverpublic.GetOTARequestObject{})
		if err != nil {
			t.Fatalf("GetOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.GetOTA404JSONResponse); !ok {
			t.Fatalf("GetOTA() response = %#v", resp)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "", "", errors.New("boom")
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.GetOTA(ctx, serverpublic.GetOTARequestObject{})
		if err != nil {
			t.Fatalf("GetOTA() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.GetOTA404JSONResponse); !ok {
			t.Fatalf("GetOTA() response = %#v", resp)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"bundles/fw.bin": "firmware"})
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			if publicKey != "peer" {
				t.Fatalf("publicKey = %q", publicKey)
			}
			return "depot", Stable, nil
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.GetOTA(ctx, serverpublic.GetOTARequestObject{})
		if err != nil {
			t.Fatalf("GetOTA() unexpected error: %v", err)
		}
		okResp, ok := resp.(serverpublic.GetOTA200JSONResponse)
		if !ok {
			t.Fatalf("GetOTA() response = %#v", resp)
		}
		if okResp.Depot != "depot" || okResp.Channel != string(Stable) || okResp.FirmwareSemver != "1.2.3" || len(okResp.Files) != 1 {
			t.Fatalf("GetOTA() payload = %+v", okResp)
		}
	})
}

func TestServerPublicDownloadFirmware(t *testing.T) {
	t.Parallel()

	t.Run("target unavailable", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		resp, err := env.srv.DownloadFirmware(context.Background(), serverpublic.DownloadFirmwareRequestObject{Path: "fw.bin"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.DownloadFirmware404JSONResponse); !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
	})

	t.Run("invalid escaped path", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.DownloadFirmware(ctx, serverpublic.DownloadFirmwareRequestObject{Path: "%"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.DownloadFirmware400JSONResponse); !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
	})

	t.Run("invalid relative path", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"bundles/fw.bin": "firmware"})
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.DownloadFirmware(ctx, serverpublic.DownloadFirmwareRequestObject{Path: "../fw.bin"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.DownloadFirmware400JSONResponse); !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"bundles/fw.bin": "firmware"})
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			return "depot", Stable, nil
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.DownloadFirmware(ctx, serverpublic.DownloadFirmwareRequestObject{Path: "missing.bin"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		if _, ok := resp.(serverpublic.DownloadFirmware404JSONResponse); !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
	})

	t.Run("open error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"bundles/fw.bin": "firmware"})
		store := newMockStore(t)
		store.base = env.store
		store.open = func(name string) (fs.File, error) { return nil, errors.New("boom") }
		srv := &Server{
			Store: store,
			ResolveGearTarget: func(ctx context.Context, publicKey string) (string, Channel, error) {
				return "depot", Stable, nil
			},
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := srv.DownloadFirmware(ctx, serverpublic.DownloadFirmwareRequestObject{Path: "bundles/fw.bin"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		if _, ok := resp.(downloadFirmware500JSONResponse); !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"bundles/fw.bin": "firmware"})
		env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
			if publicKey != "peer" {
				t.Fatalf("publicKey = %q", publicKey)
			}
			return "depot", Stable, nil
		}
		ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
		resp, err := env.srv.DownloadFirmware(ctx, serverpublic.DownloadFirmwareRequestObject{Path: "bundles/fw.bin"})
		if err != nil {
			t.Fatalf("DownloadFirmware() unexpected error: %v", err)
		}
		okResp, ok := resp.(serverpublic.DownloadFirmware200ApplicationoctetStreamResponse)
		if !ok {
			t.Fatalf("DownloadFirmware() response = %#v", resp)
		}
		data, err := io.ReadAll(okResp.Body)
		if err != nil {
			t.Fatalf("ReadAll() unexpected error: %v", err)
		}
		if string(data) != "firmware" {
			t.Fatalf("DownloadFirmware() body = %q", string(data))
		}
		if okResp.Headers.XChecksumMD5 == "" || okResp.Headers.XChecksumSHA256 == "" || okResp.ContentLength != int64(len(data)) {
			t.Fatalf("DownloadFirmware() headers = %+v length = %d", okResp.Headers, okResp.ContentLength)
		}
	})
}

func TestResolveCallerTarget(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	if _, _, err := env.srv.resolveCallerTarget(context.Background()); err == nil {
		t.Fatal("resolveCallerTarget() expected resolver error")
	}

	env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
		return "depot", Stable, nil
	}
	if _, _, err := env.srv.resolveCallerTarget(context.Background()); err == nil {
		t.Fatal("resolveCallerTarget() expected caller public key error")
	}

	env.srv.ResolveGearTarget = func(ctx context.Context, publicKey string) (string, Channel, error) {
		return "", "", nil
	}
	ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer")
	if _, _, err := env.srv.resolveCallerTarget(ctx); err == nil {
		t.Fatal("resolveCallerTarget() expected missing depot/channel error")
	}
}

func TestResolveOTAFile(t *testing.T) {
	t.Parallel()

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, _, _, err := env.srv.resolveOTAFile("depot", Stable, "../bad"); !errors.Is(err, errInvalidPath) {
			t.Fatalf("resolveOTAFile() error = %v", err)
		}
	})

	t.Run("missing depot", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		if _, _, _, err := env.srv.resolveOTAFile("missing", Stable, "fw.bin"); !errors.Is(err, errFirmwareNotFound) {
			t.Fatalf("resolveOTAFile() error = %v", err)
		}
	})

	t.Run("missing release", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeDepotInfo("depot", "fw.bin")
		if _, _, _, err := env.srv.resolveOTAFile("depot", Stable, "fw.bin"); !errors.Is(err, errFirmwareNotFound) {
			t.Fatalf("resolveOTAFile() error = %v", err)
		}
	})

	t.Run("open not exist", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"fw.bin": "firmware"})
		store := newMockStore(t)
		store.base = env.store
		store.open = func(name string) (fs.File, error) { return nil, fs.ErrNotExist }
		srv := &Server{Store: store}
		if _, _, _, err := srv.resolveOTAFile("depot", Stable, "fw.bin"); !errors.Is(err, errFirmwareNotFound) {
			t.Fatalf("resolveOTAFile() error = %v", err)
		}
	})

	t.Run("stat error", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"fw.bin": "firmware"})
		store := newMockStore(t)
		store.base = env.store
		store.open = func(name string) (fs.File, error) {
			return statErrorFile{Reader: bytes.NewReader(nil)}, nil
		}
		srv := &Server{Store: store}
		if _, _, _, err := srv.resolveOTAFile("depot", Stable, "fw.bin"); err == nil {
			t.Fatal("resolveOTAFile() expected stat error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv(t)
		env.writeRelease("depot", Stable, "1.2.3", map[string]string{"fw.bin": "firmware"})
		body, contentLength, headers, err := env.srv.resolveOTAFile("depot", Stable, "fw.bin")
		if err != nil {
			t.Fatalf("resolveOTAFile() unexpected error: %v", err)
		}
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("ReadAll() unexpected error: %v", err)
		}
		if string(data) != "firmware" || contentLength != int64(len(data)) || headers.XChecksumMD5 == "" || headers.XChecksumSHA256 == "" {
			t.Fatalf("resolveOTAFile() payload = %q length=%d headers=%+v", string(data), contentLength, headers)
		}
	})
}

func TestDownloadFirmware500Visitor(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return downloadFirmware500JSONResponse(publicError("INTERNAL_ERROR", "boom")).VisitDownloadFirmwareResponse(c)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

type statErrorFile struct {
	*bytes.Reader
}

func (f statErrorFile) Close() error               { return nil }
func (f statErrorFile) Stat() (fs.FileInfo, error) { return nil, errors.New("boom") }
