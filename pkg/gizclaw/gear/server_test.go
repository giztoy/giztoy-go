package gear

import (
	"context"
	"testing"
	"time"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

type stubPeerManager struct {
	runtime       apitypes.Runtime
	refreshResult adminservice.RefreshResult
	refreshOnline bool
	refreshErr    error
}

func (m stubPeerManager) PeerRuntime(context.Context, string) apitypes.Runtime {
	return m.runtime
}

func (m stubPeerManager) RefreshGear(context.Context, string) (adminservice.RefreshResult, bool, error) {
	return m.refreshResult, m.refreshOnline, m.refreshErr
}

func TestServerGearserviceHandlers(t *testing.T) {
	server := &Server{
		Store: mustBadgerInMemory(t, nil),
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
			Device: apitypes.DeviceInfo{
				Sn: &sn,
				Hardware: &apitypes.HardwareInfo{
					Depot:  &depot,
					Imeis:  &[]apitypes.GearIMEI{{Tac: tac, Serial: serial}},
					Labels: &[]apitypes.GearLabel{{Key: labelKey, Value: labelValue}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	getResp, err := server.GetGear(ctx, adminservice.GetGearRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGear error: %v", err)
	}
	getRegistered, ok := getResp.(adminservice.GetGear200JSONResponse)
	if !ok {
		t.Fatalf("GetGear response type = %T", getResp)
	}
	if getRegistered.PublicKey != "peer-gear" {
		t.Fatalf("GetGear = %+v", getRegistered)
	}

	listResp, err := server.ListGears(ctx, adminservice.ListGearsRequestObject{})
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	listed, ok := listResp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears response type = %T", listResp)
	}
	if len(listed.Items) != 1 || listed.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListGears items = %+v", listed.Items)
	}

	stable := apitypes.GearFirmwareChannel("stable")
	adminStable := apitypes.GearFirmwareChannel(stable)
	putConfigResp, err := server.PutGearConfig(ctx, adminservice.PutGearConfigRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
		Body: &adminservice.PutGearConfigJSONRequestBody{
			Certifications: &[]apitypes.GearCertification{{
				Type:      apitypes.GearCertificationType("license"),
				Authority: apitypes.GearCertificationAuthority("ce"),
				Id:        certID,
			}},
			Firmware: &apitypes.FirmwareConfig{Channel: &adminStable},
		},
	})
	if err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}
	if _, ok := putConfigResp.(adminservice.PutGearConfig200JSONResponse); !ok {
		t.Fatalf("PutGearConfig response type = %T", putConfigResp)
	}

	getConfigResp, err := server.GetGearConfig(ctx, adminservice.GetGearConfigRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGearConfig error: %v", err)
	}
	cfg, ok := getConfigResp.(adminservice.GetGearConfig200JSONResponse)
	if !ok {
		t.Fatalf("GetGearConfig response type = %T", getConfigResp)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != stable {
		t.Fatalf("GetGearConfig = %+v", cfg)
	}

	getInfoResp, err := server.GetGearInfo(ctx, adminservice.GetGearInfoRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("GetGearInfo error: %v", err)
	}
	info, ok := getInfoResp.(adminservice.GetGearInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetGearInfo response type = %T", getInfoResp)
	}
	if info.Hardware == nil || info.Hardware.Imeis == nil || len(*info.Hardware.Imeis) != 1 {
		t.Fatalf("GetGearInfo = %+v", info)
	}

	byFirmwareResp, err := server.ListByFirmware(ctx, adminservice.ListByFirmwareRequestObject{
		Depot:   depot,
		Channel: apitypes.GearFirmwareChannel(stable),
	})
	if err != nil {
		t.Fatalf("ListByFirmware error: %v", err)
	}
	byFirmware, ok := byFirmwareResp.(adminservice.ListByFirmware200JSONResponse)
	if !ok {
		t.Fatalf("ListByFirmware response type = %T", byFirmwareResp)
	}
	if len(byFirmware.Items) != 1 || byFirmware.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByFirmware items = %+v", byFirmware.Items)
	}

	resolveSNResp, err := server.ResolveBySN(ctx, adminservice.ResolveBySNRequestObject{Sn: sn})
	if err != nil {
		t.Fatalf("ResolveBySN error: %v", err)
	}
	resolvedSN, ok := resolveSNResp.(adminservice.ResolveBySN200JSONResponse)
	if !ok {
		t.Fatalf("ResolveBySN response type = %T", resolveSNResp)
	}
	if resolvedSN.PublicKey != "peer-gear" {
		t.Fatalf("ResolveBySN = %+v", resolvedSN)
	}

	byLabelResp, err := server.ListByLabel(ctx, adminservice.ListByLabelRequestObject{Key: labelKey, Value: labelValue})
	if err != nil {
		t.Fatalf("ListByLabel error: %v", err)
	}
	byLabel, ok := byLabelResp.(adminservice.ListByLabel200JSONResponse)
	if !ok {
		t.Fatalf("ListByLabel response type = %T", byLabelResp)
	}
	if len(byLabel.Items) != 1 || byLabel.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByLabel items = %+v", byLabel.Items)
	}

	resolveIMEIResp, err := server.ResolveByIMEI(ctx, adminservice.ResolveByIMEIRequestObject{
		Tac:    tac,
		Serial: serial,
	})
	if err != nil {
		t.Fatalf("ResolveByIMEI error: %v", err)
	}
	resolvedIMEI, ok := resolveIMEIResp.(adminservice.ResolveByIMEI200JSONResponse)
	if !ok {
		t.Fatalf("ResolveByIMEI response type = %T", resolveIMEIResp)
	}
	if resolvedIMEI.PublicKey != "peer-gear" {
		t.Fatalf("ResolveByIMEI = %+v", resolvedIMEI)
	}

	byCertificationResp, err := server.ListByCertification(ctx, adminservice.ListByCertificationRequestObject{
		Type:      apitypes.GearCertificationType("license"),
		Authority: apitypes.GearCertificationAuthority("ce"),
		Id:        certID,
	})
	if err != nil {
		t.Fatalf("ListByCertification error: %v", err)
	}
	byCertification, ok := byCertificationResp.(adminservice.ListByCertification200JSONResponse)
	if !ok {
		t.Fatalf("ListByCertification response type = %T", byCertificationResp)
	}
	if len(byCertification.Items) != 1 || byCertification.Items[0].PublicKey != "peer-gear" {
		t.Fatalf("ListByCertification items = %+v", byCertification.Items)
	}

	approveResp, err := server.ApproveGear(ctx, adminservice.ApproveGearRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
		Body:      &adminservice.ApproveGearJSONRequestBody{Role: apitypes.GearRoleDevice},
	})
	if err != nil {
		t.Fatalf("ApproveGear error: %v", err)
	}
	approved, ok := approveResp.(adminservice.ApproveGear200JSONResponse)
	if !ok {
		t.Fatalf("ApproveGear response type = %T", approveResp)
	}
	if approved.Role != apitypes.GearRoleDevice || approved.Status != apitypes.GearStatusActive {
		t.Fatalf("ApproveGear = %+v", approved)
	}

	blockResp, err := server.BlockGear(ctx, adminservice.BlockGearRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("BlockGear error: %v", err)
	}
	blocked, ok := blockResp.(adminservice.BlockGear200JSONResponse)
	if !ok {
		t.Fatalf("BlockGear response type = %T", blockResp)
	}
	if blocked.Status != apitypes.GearStatusBlocked {
		t.Fatalf("BlockGear = %+v", blocked)
	}

	deleteResp, err := server.DeleteGear(ctx, adminservice.DeleteGearRequestObject{
		PublicKey: adminservice.PublicKey("peer-gear"),
	})
	if err != nil {
		t.Fatalf("DeleteGear error: %v", err)
	}
	deleted, ok := deleteResp.(adminservice.DeleteGear200JSONResponse)
	if !ok {
		t.Fatalf("DeleteGear response type = %T", deleteResp)
	}
	if deleted.Role != apitypes.GearRoleUnspecified || deleted.Status != apitypes.GearStatusUnspecified || deleted.ApprovedAt != nil {
		t.Fatalf("DeleteGear = %+v", deleted)
	}
}

func TestServerListGearsPagination(t *testing.T) {
	server := &Server{
		Store: mustBadgerInMemory(t, nil),
	}

	registerGear := func(publicKey, labelValue string) {
		ctx := serverpublic.WithCallerPublicKey(context.Background(), publicKey)
		_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
			Body: &serverpublic.RegisterGearJSONRequestBody{
				Device: apitypes.DeviceInfo{
					Hardware: &apitypes.HardwareInfo{
						Labels: &[]apitypes.GearLabel{{Key: "region", Value: labelValue}},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("RegisterGear(%q) error: %v", publicKey, err)
		}
	}

	registerGear("gear-a", "cn")
	registerGear("gear-b", "cn")
	registerGear("gear-c", "us")

	limit := adminservice.Limit(1)
	resp, err := server.ListGears(context.Background(), adminservice.ListGearsRequestObject{
		Params: adminservice.ListGearsParams{
			Limit: &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListGears pagination error: %v", err)
	}
	listed, ok := resp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears response type = %T", resp)
	}
	if !listed.HasNext || listed.NextCursor == nil || *listed.NextCursor != "gear-a" {
		t.Fatalf("ListGears pagination metadata = %+v", listed)
	}
	if len(listed.Items) != 1 || listed.Items[0].PublicKey != "gear-a" {
		t.Fatalf("ListGears paged items = %+v", listed.Items)
	}

	firstFilteredResp, err := server.ListByLabel(context.Background(), adminservice.ListByLabelRequestObject{
		Key:   "region",
		Value: "cn",
		Params: adminservice.ListByLabelParams{
			Limit: &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListByLabel pagination error: %v", err)
	}
	firstFiltered, ok := firstFilteredResp.(adminservice.ListByLabel200JSONResponse)
	if !ok {
		t.Fatalf("ListByLabel response type = %T", firstFilteredResp)
	}
	if !firstFiltered.HasNext || firstFiltered.NextCursor == nil || *firstFiltered.NextCursor != "gear-a" {
		t.Fatalf("ListByLabel first page metadata = %+v", firstFiltered)
	}

	filteredResp, err := server.ListByLabel(context.Background(), adminservice.ListByLabelRequestObject{
		Key:   "region",
		Value: "cn",
		Params: adminservice.ListByLabelParams{
			Cursor: firstFiltered.NextCursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListByLabel second page error: %v", err)
	}
	filtered, ok := filteredResp.(adminservice.ListByLabel200JSONResponse)
	if !ok {
		t.Fatalf("ListByLabel second response type = %T", filteredResp)
	}
	if filtered.HasNext || filtered.NextCursor != nil {
		t.Fatalf("ListByLabel pagination metadata = %+v", filtered)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].PublicKey != "gear-b" {
		t.Fatalf("ListByLabel paged items = %+v", filtered.Items)
	}
}

func TestServerListGearsPaginationPreservesCreationOrder(t *testing.T) {
	server := &Server{
		Store: mustBadgerInMemory(t, nil),
	}

	registerGear := func(publicKey string) {
		ctx := serverpublic.WithCallerPublicKey(context.Background(), publicKey)
		_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
			Body: &serverpublic.RegisterGearJSONRequestBody{
				Device: apitypes.DeviceInfo{},
			},
		})
		if err != nil {
			t.Fatalf("RegisterGear(%q) error: %v", publicKey, err)
		}
	}

	registerGear("gear-b")
	registerGear("gear-a")
	registerGear("gear-c")

	limit := adminservice.Limit(2)
	resp, err := server.ListGears(context.Background(), adminservice.ListGearsRequestObject{
		Params: adminservice.ListGearsParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListGears first page error: %v", err)
	}
	firstPage, ok := resp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears first response type = %T", resp)
	}
	if len(firstPage.Items) != 2 || firstPage.Items[0].PublicKey != "gear-b" || firstPage.Items[1].PublicKey != "gear-a" {
		t.Fatalf("ListGears first page = %+v", firstPage.Items)
	}
	if !firstPage.HasNext || firstPage.NextCursor == nil || *firstPage.NextCursor != "gear-a" {
		t.Fatalf("ListGears first page metadata = %+v", firstPage)
	}

	resp, err = server.ListGears(context.Background(), adminservice.ListGearsRequestObject{
		Params: adminservice.ListGearsParams{
			Cursor: firstPage.NextCursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListGears second page error: %v", err)
	}
	secondPage, ok := resp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears second response type = %T", resp)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].PublicKey != "gear-c" {
		t.Fatalf("ListGears second page = %+v", secondPage.Items)
	}
}

func TestServerListGearsLimitClampsToConfiguredBounds(t *testing.T) {
	server := &Server{
		Store: mustBadgerInMemory(t, nil),
	}
	for _, publicKey := range []string{"gear-a", "gear-b", "gear-c"} {
		ctx := serverpublic.WithCallerPublicKey(context.Background(), publicKey)
		_, err := server.RegisterGear(ctx, serverpublic.RegisterGearRequestObject{
			Body: &serverpublic.RegisterGearJSONRequestBody{Device: apitypes.DeviceInfo{}},
		})
		if err != nil {
			t.Fatalf("RegisterGear(%q) error: %v", publicKey, err)
		}
	}

	zero := adminservice.Limit(0)
	resp, err := server.ListGears(context.Background(), adminservice.ListGearsRequestObject{
		Params: adminservice.ListGearsParams{Limit: &zero},
	})
	if err != nil {
		t.Fatalf("ListGears zero limit error: %v", err)
	}
	defaultPage, ok := resp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears zero limit response type = %T", resp)
	}
	if len(defaultPage.Items) != 3 || defaultPage.HasNext {
		t.Fatalf("ListGears zero limit = %+v", defaultPage)
	}

	tooLarge := adminservice.Limit(999)
	resp, err = server.ListGears(context.Background(), adminservice.ListGearsRequestObject{
		Params: adminservice.ListGearsParams{Limit: &tooLarge},
	})
	if err != nil {
		t.Fatalf("ListGears large limit error: %v", err)
	}
	clampedPage, ok := resp.(adminservice.ListGears200JSONResponse)
	if !ok {
		t.Fatalf("ListGears large limit response type = %T", resp)
	}
	if len(clampedPage.Items) != 3 || clampedPage.HasNext {
		t.Fatalf("ListGears large limit = %+v", clampedPage)
	}
}

func TestServerRuntimeHandlers(t *testing.T) {
	now := time.Unix(1_700_200_000, 0).UTC()
	runtimeAddr := "10.0.0.1:1234"
	server := &Server{
		Store: mustBadgerInMemory(t, nil),
		PeerManager: stubPeerManager{
			runtime: apitypes.Runtime{
				LastAddr:   &runtimeAddr,
				LastSeenAt: now,
				Online:     true,
			},
			refreshResult: adminservice.RefreshResult{
				Gear: apitypes.Gear{
					PublicKey: "peer-3",
					Role:      apitypes.GearRolePeer,
					Status:    apitypes.GearStatusActive,
				},
				UpdatedFields: &[]string{"device.name"},
			},
			refreshOnline: true,
		},
	}

	registerCtx := serverpublic.WithCallerPublicKey(context.Background(), "peer-3")
	gearCtx := gearservice.WithCallerPublicKey(context.Background(), "peer-3")
	_, err := server.RegisterGear(registerCtx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{Device: apitypes.DeviceInfo{}},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	getGearRuntimeResp, err := server.GetGearRuntime(context.Background(), adminservice.GetGearRuntimeRequestObject{
		PublicKey: adminservice.PublicKey("peer-3"),
	})
	if err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	gearRuntime, ok := getGearRuntimeResp.(adminservice.GetGearRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetGearRuntime response type = %T", getGearRuntimeResp)
	}
	if !gearRuntime.Online || gearRuntime.LastAddr == nil || *gearRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetGearRuntime = %+v", gearRuntime)
	}

	getRuntimeResp, err := server.GetRuntime(gearCtx, gearservice.GetRuntimeRequestObject{})
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	publicRuntime, ok := getRuntimeResp.(gearservice.GetRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetRuntime response type = %T", getRuntimeResp)
	}
	if !publicRuntime.Online || publicRuntime.LastAddr == nil || *publicRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetRuntime = %+v", publicRuntime)
	}

	refreshResp, err := server.RefreshGear(context.Background(), adminservice.RefreshGearRequestObject{
		PublicKey: adminservice.PublicKey("peer-3"),
	})
	if err != nil {
		t.Fatalf("RefreshGear error: %v", err)
	}
	refreshed, ok := refreshResp.(adminservice.RefreshGear200JSONResponse)
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
		Store:              mustBadgerInMemory(t, nil),
		RegistrationTokens: map[string]apitypes.GearRole{"admin": apitypes.GearRoleAdmin},
		BuildCommit:        "deadbeef",
		ServerPublicKey:    "server-pk",
	}

	registerCtx := serverpublic.WithCallerPublicKey(context.Background(), "peer-1")
	gearCtx := gearservice.WithCallerPublicKey(context.Background(), "peer-1")
	token := "admin"
	name := "gear-a"
	sn := "sn-1"
	depot := "alpha"
	labelKey := "region"
	labelValue := "cn"

	registerResp, err := server.RegisterGear(registerCtx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{
			RegistrationToken: &token,
			Device: apitypes.DeviceInfo{
				Name: &name,
				Sn:   &sn,
				Hardware: &apitypes.HardwareInfo{
					Depot:  &depot,
					Labels: &[]apitypes.GearLabel{{Key: labelKey, Value: labelValue}},
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
	if registered.Registration.Role != apitypes.GearRole(apitypes.GearRoleAdmin) {
		t.Fatalf("Role = %q", registered.Registration.Role)
	}
	if registered.Registration.Status != apitypes.GearStatus(apitypes.GearStatusActive) {
		t.Fatalf("Status = %q", registered.Registration.Status)
	}
	if registered.Registration.ApprovedAt == nil || registered.Registration.ApprovedAt.Before(before) || registered.Registration.ApprovedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("ApprovedAt = %v", registered.Registration.ApprovedAt)
	}

	getInfoResp, err := server.GetInfo(gearCtx, gearservice.GetInfoRequestObject{})
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	info, ok := getInfoResp.(gearservice.GetInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetInfo response type = %T", getInfoResp)
	}
	if info.Sn == nil || *info.Sn != sn {
		t.Fatalf("GetInfo sn = %v", info.Sn)
	}

	getRegistrationResp, err := server.GetRegistration(gearCtx, gearservice.GetRegistrationRequestObject{})
	if err != nil {
		t.Fatalf("GetRegistration error: %v", err)
	}
	publicRegistration, ok := getRegistrationResp.(gearservice.GetRegistration200JSONResponse)
	if !ok {
		t.Fatalf("GetRegistration response type = %T", getRegistrationResp)
	}
	if publicRegistration.Role != apitypes.GearRole(apitypes.GearRoleAdmin) {
		t.Fatalf("GetRegistration role = %q", publicRegistration.Role)
	}

	serverInfoResp, err := server.GetServerInfo(registerCtx, serverpublic.GetServerInfoRequestObject{})
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
		Store: mustBadgerInMemory(t, nil),
		PeerManager: stubPeerManager{
			runtime: apitypes.Runtime{
				LastAddr:   &runtimeAddr,
				LastSeenAt: now,
				Online:     true,
			},
		},
	}

	registerCtx := serverpublic.WithCallerPublicKey(context.Background(), "peer-public")
	gearCtx := gearservice.WithCallerPublicKey(context.Background(), "peer-public")
	sn := "sn-old"
	depot := "depot-public"
	_, err := server.RegisterGear(registerCtx, serverpublic.RegisterGearRequestObject{
		Body: &serverpublic.RegisterGearJSONRequestBody{
			Device: apitypes.DeviceInfo{
				Sn: &sn,
				Hardware: &apitypes.HardwareInfo{
					Depot: &depot,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}

	stable := apitypes.GearFirmwareChannel("stable")
	adminStable := apitypes.GearFirmwareChannel(stable)
	_, err = server.PutGearConfig(context.Background(), adminservice.PutGearConfigRequestObject{
		PublicKey: adminservice.PublicKey("peer-public"),
		Body: &adminservice.PutGearConfigJSONRequestBody{
			Firmware: &apitypes.FirmwareConfig{Channel: &adminStable},
		},
	})
	if err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}

	getConfigResp, err := server.GetConfig(gearCtx, gearservice.GetConfigRequestObject{})
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	cfg, ok := getConfigResp.(gearservice.GetConfig200JSONResponse)
	if !ok {
		t.Fatalf("GetConfig response type = %T", getConfigResp)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != stable {
		t.Fatalf("GetConfig = %+v", cfg)
	}

	newSN := "sn-new"
	putInfoResp, err := server.PutInfo(gearCtx, gearservice.PutInfoRequestObject{
		Body: &gearservice.PutInfoJSONRequestBody{
			Sn: &newSN,
			Hardware: &apitypes.HardwareInfo{
				Depot: &depot,
			},
		},
	})
	if err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	putInfo, ok := putInfoResp.(gearservice.PutInfo200JSONResponse)
	if !ok {
		t.Fatalf("PutInfo response type = %T", putInfoResp)
	}
	if putInfo.Sn == nil || *putInfo.Sn != newSN {
		t.Fatalf("PutInfo = %+v", putInfo)
	}

	getRuntimeResp, err := server.GetRuntime(gearCtx, gearservice.GetRuntimeRequestObject{})
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	publicRuntime, ok := getRuntimeResp.(gearservice.GetRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetRuntime response type = %T", getRuntimeResp)
	}
	if !publicRuntime.Online || publicRuntime.LastAddr == nil || *publicRuntime.LastAddr != runtimeAddr {
		t.Fatalf("GetRuntime = %+v", publicRuntime)
	}
}
