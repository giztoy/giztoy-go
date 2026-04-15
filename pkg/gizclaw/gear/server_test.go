package gear

import (
	"context"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

type stubPeerManager struct {
	runtime       gearservice.Runtime
	refreshResult gearservice.RefreshResult
	refreshOnline bool
	refreshErr    error
}

func (m stubPeerManager) PeerRuntime(context.Context, string) gearservice.Runtime {
	return m.runtime
}

func (m stubPeerManager) RefreshGear(context.Context, string) (gearservice.RefreshResult, bool, error) {
	return m.refreshResult, m.refreshOnline, m.refreshErr
}

func TestServerGearserviceHandlers(t *testing.T) {
	server := &Server{
		Store: kv.NewMemory(nil),
	}

	ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer-gear")
	sn := "sn-gear"
	depot := "depot-gear"
	tac := "12345678"
	serial := "87654321"
	labelKey := "region"
	labelValue := "cn"
	certID := "cert-gear"

	_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{
			Device: serverpublic.DeviceInfo{
				Sn: &sn,
				Hardware: &serverpublic.HardwareInfo{
					Depot:  &depot,
					Imeis:  &[]serverpublic.GearIMEI{{Tac: tac, Serial: serial}},
					Labels: &[]serverpublic.GearLabel{{Key: labelKey, Value: labelValue}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	getResp, err := server.GetGear(ctx, gearservice.GetGearRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGear error: %v", err)
	}
	getRegistered, ok := getResp.(gearservice.GetGear200JSONResponse)
	if !ok {
		t.Fatalf("GetGear response type = %T", getResp)
	}
	if getRegistered.PublicKey != "peer-gear" {
		t.Fatalf("GetGear = %+v", getRegistered)
	}

	listResp, err := server.ListGears(ctx, gearservice.ListGearsRequestObject{})
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	listed, ok := listResp.(gearservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears response type = %T", listResp)
	}
	if len(listed.Items) != 1 || listed.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListGears items = %+v", listed.Items)
	}

	stable := gearservice.GearFirmwareChannel("stable")
	putConfigResp, err := server.PutGearConfig(ctx, gearservice.PutGearConfigRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
		Body: &gearservice.PutGearConfigJSONRequestBody{
			Certifications: &[]gearservice.GearCertification{{
				Type:      gearservice.GearCertificationType("license"),
				Authority: gearservice.GearCertificationAuthority("ce"),
				Id:        certID,
			}},
			Firmware: &gearservice.FirmwareConfig{Channel: &stable},
		},
	})
	if err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}
	if _, ok := putConfigResp.(gearservice.PutGearConfig200JSONResponse); !ok {
		t.Fatalf("PutGearConfig response type = %T", putConfigResp)
	}

	getConfigResp, err := server.GetGearConfig(ctx, gearservice.GetGearConfigRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGearConfig error: %v", err)
	}
	cfg, ok := getConfigResp.(gearservice.GetGearConfig200JSONResponse)
	if !ok {
		t.Fatalf("GetGearConfig response type = %T", getConfigResp)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != stable {
		t.Fatalf("GetGearConfig = %+v", cfg)
	}

	getInfoResp, err := server.GetGearInfo(ctx, gearservice.GetGearInfoRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGearInfo error: %v", err)
	}
	info, ok := getInfoResp.(gearservice.GetGearInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetGearInfo response type = %T", getInfoResp)
	}
	if info.Hardware == nil || info.Hardware.Imeis == nil || len(*info.Hardware.Imeis) != 1 {
		t.Fatalf("GetGearInfo = %+v", info)
	}

	byFirmwareResp, err := server.ListByFirmware(ctx, gearservice.ListByFirmwareRequestObject{
		Depot:   depot,
		Channel: stable,
	})
	if err != nil {
		t.Fatalf("ListByFirmware error: %v", err)
	}
	byFirmware, ok := byFirmwareResp.(gearservice.ListByFirmware200JSONResponse)
	if !ok {
		t.Fatalf("ListByFirmware response type = %T", byFirmwareResp)
	}
	if len(byFirmware.Items) != 1 || byFirmware.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByFirmware items = %+v", byFirmware.Items)
	}

	resolveSNResp, err := server.ResolveBySN(ctx, gearservice.ResolveBySNRequestObject{Sn: sn})
	if err != nil {
		t.Fatalf("ResolveBySN error: %v", err)
	}
	resolvedSN, ok := resolveSNResp.(gearservice.ResolveBySN200JSONResponse)
	if !ok {
		t.Fatalf("ResolveBySN response type = %T", resolveSNResp)
	}
	if resolvedSN.PublicKey != "peer-gear" {
		t.Fatalf("ResolveBySN = %+v", resolvedSN)
	}

	byLabelResp, err := server.ListByLabel(ctx, gearservice.ListByLabelRequestObject{Key: labelKey, Value: labelValue})
	if err != nil {
		t.Fatalf("ListByLabel error: %v", err)
	}
	byLabel, ok := byLabelResp.(gearservice.ListByLabel200JSONResponse)
	if !ok {
		t.Fatalf("ListByLabel response type = %T", byLabelResp)
	}
	if len(byLabel.Items) != 1 || byLabel.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByLabel items = %+v", byLabel.Items)
	}

	resolveIMEIResp, err := server.ResolveByIMEI(ctx, gearservice.ResolveByIMEIRequestObject{
		Tac:    tac,
		Serial: serial,
	})
	if err != nil {
		t.Fatalf("ResolveByIMEI error: %v", err)
	}
	resolvedIMEI, ok := resolveIMEIResp.(gearservice.ResolveByIMEI200JSONResponse)
	if !ok {
		t.Fatalf("ResolveByIMEI response type = %T", resolveIMEIResp)
	}
	if resolvedIMEI.PublicKey != "peer-gear" {
		t.Fatalf("ResolveByIMEI = %+v", resolvedIMEI)
	}

	byCertificationResp, err := server.ListByCertification(ctx, gearservice.ListByCertificationRequestObject{
		Type:      gearservice.GearCertificationType("license"),
		Authority: gearservice.GearCertificationAuthority("ce"),
		Id:        certID,
	})
	if err != nil {
		t.Fatalf("ListByCertification error: %v", err)
	}
	byCertification, ok := byCertificationResp.(gearservice.ListByCertification200JSONResponse)
	if !ok {
		t.Fatalf("ListByCertification response type = %T", byCertificationResp)
	}
	if len(byCertification.Items) != 1 || byCertification.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByCertification items = %+v", byCertification.Items)
	}

	approveResp, err := server.ApproveGear(ctx, gearservice.ApproveGearRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
		Body:      &gearservice.ApproveGearJSONRequestBody{Role: gearservice.GearRoleDevice},
	})
	if err != nil {
		t.Fatalf("ApproveGear error: %v", err)
	}
	approved, ok := approveResp.(gearservice.ApproveGear200JSONResponse)
	if !ok {
		t.Fatalf("ApproveGear response type = %T", approveResp)
	}
	if approved.Role != gearservice.GearRoleDevice || approved.Status != gearservice.GearStatusActive {
		t.Fatalf("ApproveGear = %+v", approved)
	}

	blockResp, err := server.BlockGear(ctx, gearservice.BlockGearRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("BlockGear error: %v", err)
	}
	blocked, ok := blockResp.(gearservice.BlockGear200JSONResponse)
	if !ok {
		t.Fatalf("BlockGear response type = %T", blockResp)
	}
	if blocked.Status != gearservice.GearStatusBlocked {
		t.Fatalf("BlockGear = %+v", blocked)
	}

	deleteResp, err := server.DeleteGear(ctx, gearservice.DeleteGearRequestObject{
		PublicKey: gearservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("DeleteGear error: %v", err)
	}
	deleted, ok := deleteResp.(gearservice.DeleteGear200JSONResponse)
	if !ok {
		t.Fatalf("DeleteGear response type = %T", deleteResp)
	}
	if deleted.Role != gearservice.GearRoleUnspecified || deleted.Status != gearservice.GearStatusUnspecified || deleted.ApprovedAt != nil {
		t.Fatalf("DeleteGear = %+v", deleted)
	}
}

func TestServerRuntimeHandlers(t *testing.T) {
	now := time.Unix(1_700_200_000, 0).UTC()
	runtimeAddr := "10.0.0.1:1234"
	server := &Server{
		Store: kv.NewMemory(nil),
		PeerManager: stubPeerManager{
			runtime: gearservice.Runtime{
				LastAddr:   &runtimeAddr,
				LastSeenAt: now,
				Online:     true,
			},
			refreshResult: gearservice.RefreshResult{
				Gear: gearservice.Gear{
					PublicKey: "peer-3",
					Role:      gearservice.GearRolePeer,
					Status:    gearservice.GearStatusActive,
				},
				UpdatedFields: &[]string{"device.name"},
			},
			refreshOnline: true,
		},
	}

	ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer-3")
	_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{Device: serverpublic.DeviceInfo{}},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	getGearRuntimeResp, err := server.GetGearRuntime(ctx, gearservice.GetGearRuntimeRequestObject{
		PublicKey: gearservice.PublicKey("peer-3"),
	})
	if err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	gearRuntime, ok := getGearRuntimeResp.(gearservice.GetGearRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetGearRuntime response type = %T", getGearRuntimeResp)
	}
	if !gearRuntime.Online || gearRuntime.LastAddr == nil || *gearRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetGearRuntime = %+v", gearRuntime)
	}

	getRuntimeResp, err := server.GetRuntime(ctx, serverpublic.GetRuntimeRequestObject{})
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	publicRuntime, ok := getRuntimeResp.(serverpublic.GetRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetRuntime response type = %T", getRuntimeResp)
	}
	if !publicRuntime.Online || publicRuntime.LastAddr == nil || *publicRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetRuntime = %+v", publicRuntime)
	}

	refreshResp, err := server.RefreshGear(ctx, gearservice.RefreshGearRequestObject{
		PublicKey: gearservice.PublicKey("peer-3"),
	})
	if err != nil {
		t.Fatalf("RefreshGear error: %v", err)
	}
	refreshed, ok := refreshResp.(gearservice.RefreshGear200JSONResponse)
	if !ok {
		t.Fatalf("RefreshGear response type = %T", refreshResp)
	}
	if refreshed.Gear.PublicKey != "peer-3" || refreshed.UpdatedFields == nil || len(*refreshed.UpdatedFields) != 1 {
		t.Fatalf("RefreshGear = %+v", refreshed)
	}
}

func TestServerPublicHandlers(t *testing.T) {
	before := time.Now()
	server := &Server{
		Store:              kv.NewMemory(nil),
		RegistrationTokens: map[string]gearservice.GearRole{"admin": gearservice.GearRoleAdmin},
		BuildCommit:        "deadbeef",
		ServerPublicKey:    "server-pk",
	}

	ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer-1")
	token := "admin"
	name := "gear-a"
	sn := "sn-1"
	depot := "alpha"
	labelKey := "region"
	labelValue := "cn"

	registerResp, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{
			RegistrationToken: &token,
			Device: serverpublic.DeviceInfo{
				Name: &name,
				Sn:   &sn,
				Hardware: &serverpublic.HardwareInfo{
					Depot:  &depot,
					Labels: &[]serverpublic.GearLabel{{Key: labelKey, Value: labelValue}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	registered, ok := registerResp.(serverpublic.RegisterGear200JSONResponse)
	if !ok {
		t.Fatalf("RegisterGear response type = %T", registerResp)
	}
	if registered.Registration.PublicKey != "peer-1" {
		t.Fatalf("PublicKey = %q", registered.Registration.PublicKey)
	}
	if registered.Registration.Role != serverpublic.GearRole(gearservice.GearRoleAdmin) {
		t.Fatalf("Role = %q", registered.Registration.Role)
	}
	if registered.Registration.Status != serverpublic.GearStatus(gearservice.GearStatusActive) {
		t.Fatalf("Status = %q", registered.Registration.Status)
	}
	if registered.Registration.ApprovedAt == nil || registered.Registration.ApprovedAt.Before(before) || registered.Registration.ApprovedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("ApprovedAt = %v", registered.Registration.ApprovedAt)
	}

	getInfoResp, err := server.GetInfo(ctx, serverpublic.GetInfoRequestObject{})
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	info, ok := getInfoResp.(serverpublic.GetInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetInfo response type = %T", getInfoResp)
	}
	if info.Sn == nil || *info.Sn != sn {
		t.Fatalf("GetInfo sn = %v", info.Sn)
	}

	getRegistrationResp, err := server.GetRegistration(ctx, serverpublic.GetRegistrationRequestObject{})
	if err != nil {
		t.Fatalf("GetRegistration error: %v", err)
	}
	publicRegistration, ok := getRegistrationResp.(serverpublic.GetRegistration200JSONResponse)
	if !ok {
		t.Fatalf("GetRegistration response type = %T", getRegistrationResp)
	}
	if publicRegistration.Role != serverpublic.GearRole(gearservice.GearRoleAdmin) {
		t.Fatalf("GetRegistration role = %q", publicRegistration.Role)
	}

	serverInfoResp, err := server.GetServerInfo(ctx, serverpublic.GetServerInfoRequestObject{})
	if err != nil {
		t.Fatalf("GetServerInfo error: %v", err)
	}
	serverInfo, ok := serverInfoResp.(serverpublic.GetServerInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetServerInfo response type = %T", serverInfoResp)
	}
	if serverInfo.BuildCommit != "deadbeef" || serverInfo.PublicKey != "server-pk" {
		t.Fatalf("GetServerInfo = %+v", serverInfo)
	}
	if serverInfo.ServerTime < before.UnixMilli() || serverInfo.ServerTime > time.Now().Add(time.Second).UnixMilli() {
		t.Fatalf("GetServerInfo = %+v", serverInfo)
	}
}

func TestServerPublicHandlersPutInfoConfigAndRuntime(t *testing.T) {
	now := time.Unix(1_700_500_000, 0).UTC()
	runtimeAddr := "10.0.0.1:8888"
	server := &Server{
		Store: kv.NewMemory(nil),
		PeerManager: stubPeerManager{
			runtime: gearservice.Runtime{
				LastAddr:   &runtimeAddr,
				LastSeenAt: now,
				Online:     true,
			},
		},
	}

	ctx := serverpublic.WithCallerPublicKey(context.Background(), "peer-public")
	sn := "sn-old"
	depot := "depot-public"
	_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{
			Device: serverpublic.DeviceInfo{
				Sn: &sn,
				Hardware: &serverpublic.HardwareInfo{
					Depot: &depot,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	stable := gearservice.GearFirmwareChannel("stable")
	_, err = server.PutGearConfig(ctx, gearservice.PutGearConfigRequestObject{
		PublicKey: gearservice.PublicKey("peer-public"),
		Body: &gearservice.PutGearConfigJSONRequestBody{
			Firmware: &gearservice.FirmwareConfig{Channel: &stable},
		},
	})
	if err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}

	getConfigResp, err := server.GetConfig(ctx, serverpublic.GetConfigRequestObject{})
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	cfg, ok := getConfigResp.(serverpublic.GetConfig200JSONResponse)
	if !ok {
		t.Fatalf("GetConfig response type = %T", getConfigResp)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != serverpublic.GearFirmwareChannel(stable) {
		t.Fatalf("GetConfig = %+v", cfg)
	}

	newSN := "sn-new"
	putInfoResp, err := server.PutInfo(ctx, serverpublic.PutInfoRequestObject{
		Body: &serverpublic.PutInfoJSONRequestBody{
			Sn: &newSN,
			Hardware: &serverpublic.HardwareInfo{
				Depot: &depot,
			},
		},
	})
	if err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	putInfo, ok := putInfoResp.(serverpublic.PutInfo200JSONResponse)
	if !ok {
		t.Fatalf("PutInfo response type = %T", putInfoResp)
	}
	if putInfo.Sn == nil || *putInfo.Sn != newSN {
		t.Fatalf("PutInfo = %+v", putInfo)
	}

	getRuntimeResp, err := server.GetRuntime(ctx, serverpublic.GetRuntimeRequestObject{})
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	publicRuntime, ok := getRuntimeResp.(serverpublic.GetRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetRuntime response type = %T", getRuntimeResp)
	}
	if !publicRuntime.Online || publicRuntime.LastAddr == nil || *publicRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetRuntime = %+v", publicRuntime)
	}
}
