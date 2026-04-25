package mmx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestServerMiniMaxTenantsCRUD(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.Token,
		Body:      apitypes.CredentialBody{"token": "tok-main"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})

	createBody := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main",
		"base_url": "https://api.minimax.chat",
		"description": "primary tenant"
	}`)
	createResp, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateMiniMaxTenant() error = %v", err)
	}
	created, ok := createResp.(adminservice.CreateMiniMaxTenant200JSONResponse)
	if !ok {
		t.Fatalf("CreateMiniMaxTenant() response = %#v", createResp)
	}
	if created.Name != "tenant-a" || created.CredentialName != "cred-main" {
		t.Fatalf("CreateMiniMaxTenant() tenant = %#v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("CreateMiniMaxTenant() timestamps = %#v", created)
	}

	getResp, err := srv.GetMiniMaxTenant(ctx, adminservice.GetMiniMaxTenantRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("GetMiniMaxTenant() error = %v", err)
	}
	got, ok := getResp.(adminservice.GetMiniMaxTenant200JSONResponse)
	if !ok {
		t.Fatalf("GetMiniMaxTenant() response = %#v", getResp)
	}
	if got.AppId != "app-1" || got.GroupId != "group-1" {
		t.Fatalf("GetMiniMaxTenant() tenant = %#v", got)
	}

	updateBody := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-2",
		"group_id": "group-2",
		"credential_name": "cred-main",
		"description": "updated tenant"
	}`)
	putResp, err := srv.PutMiniMaxTenant(ctx, adminservice.PutMiniMaxTenantRequestObject{
		Name: "tenant-a",
		Body: &updateBody,
	})
	if err != nil {
		t.Fatalf("PutMiniMaxTenant() error = %v", err)
	}
	updated, ok := putResp.(adminservice.PutMiniMaxTenant200JSONResponse)
	if !ok {
		t.Fatalf("PutMiniMaxTenant() response = %#v", putResp)
	}
	if updated.CreatedAt != created.CreatedAt {
		t.Fatalf("PutMiniMaxTenant() created_at = %v, want %v", updated.CreatedAt, created.CreatedAt)
	}
	if updated.AppId != "app-2" || updated.GroupId != "group-2" {
		t.Fatalf("PutMiniMaxTenant() tenant = %#v", updated)
	}

	listResp, err := srv.ListMiniMaxTenants(ctx, adminservice.ListMiniMaxTenantsRequestObject{})
	if err != nil {
		t.Fatalf("ListMiniMaxTenants() error = %v", err)
	}
	listed, ok := listResp.(adminservice.ListMiniMaxTenants200JSONResponse)
	if !ok {
		t.Fatalf("ListMiniMaxTenants() response = %#v", listResp)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "tenant-a" {
		t.Fatalf("ListMiniMaxTenants() = %#v", listed)
	}

	voice := apitypes.Voice{
		CreatedAt: created.CreatedAt,
		Id:        "minimax-tenant:tenant-a:voice-1",
		Provider: apitypes.VoiceProvider{
			Kind: miniMaxVoiceProviderKind,
			Name: apitypes.VoiceProviderName("tenant-a"),
		},
		ProviderVoiceId: stringPtr("voice-1"),
		Source:          apitypes.Sync,
		SyncedAt:        timePtr(created.CreatedAt),
		UpdatedAt:       created.CreatedAt,
	}
	if err := writeVoice(ctx, srv.Store, voice, nil); err != nil {
		t.Fatalf("writeVoice() error = %v", err)
	}
	manualVoice := apitypes.Voice{
		CreatedAt: created.CreatedAt,
		Id:        "manual:tenant-a:voice-2",
		Provider: apitypes.VoiceProvider{
			Kind: miniMaxVoiceProviderKind,
			Name: apitypes.VoiceProviderName("tenant-a"),
		},
		Source:    apitypes.Manual,
		UpdatedAt: created.CreatedAt,
	}
	if err := writeVoice(ctx, srv.Store, manualVoice, nil); err != nil {
		t.Fatalf("writeVoice(manual) error = %v", err)
	}

	deleteResp, err := srv.DeleteMiniMaxTenant(ctx, adminservice.DeleteMiniMaxTenantRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("DeleteMiniMaxTenant() error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteMiniMaxTenant200JSONResponse); !ok {
		t.Fatalf("DeleteMiniMaxTenant() response = %#v", deleteResp)
	}
	if _, err := getVoice(ctx, srv.Store, string(voice.Id)); err != kv.ErrNotFound {
		t.Fatalf("getVoice() after tenant delete err = %v, want kv.ErrNotFound", err)
	}
	if _, err := getVoice(ctx, srv.Store, string(manualVoice.Id)); err != nil {
		t.Fatalf("manual voice after tenant delete err = %v, want nil", err)
	}
}

func TestServerMiniMaxTenantsPaginationAndValidation(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.Token,
		Body:      apitypes.CredentialBody{"token": "tok-main"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})

	for _, body := range []adminservice.MiniMaxTenantUpsert{
		{Name: "alpha", AppId: "app-a", GroupId: "group-a", CredentialName: "cred-main"},
		{Name: "beta", AppId: "app-b", GroupId: "group-b", CredentialName: "cred-main"},
		{Name: "gamma", AppId: "app-c", GroupId: "group-c", CredentialName: "cred-main"},
	} {
		if _, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &body}); err != nil {
			t.Fatalf("CreateMiniMaxTenant(%q) error = %v", body.Name, err)
		}
	}

	limit := adminservice.Limit(1)
	firstResp, err := srv.ListMiniMaxTenants(ctx, adminservice.ListMiniMaxTenantsRequestObject{
		Params: adminservice.ListMiniMaxTenantsParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListMiniMaxTenants(first page) error = %v", err)
	}
	first, ok := firstResp.(adminservice.ListMiniMaxTenants200JSONResponse)
	if !ok {
		t.Fatalf("ListMiniMaxTenants(first page) response = %#v", firstResp)
	}
	if len(first.Items) != 1 || !first.HasNext || first.NextCursor == nil {
		t.Fatalf("ListMiniMaxTenants(first page) = %#v", first)
	}

	cursor := adminservice.Cursor(*first.NextCursor)
	secondResp, err := srv.ListMiniMaxTenants(ctx, adminservice.ListMiniMaxTenantsRequestObject{
		Params: adminservice.ListMiniMaxTenantsParams{
			Cursor: &cursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListMiniMaxTenants(second page) error = %v", err)
	}
	second, ok := secondResp.(adminservice.ListMiniMaxTenants200JSONResponse)
	if !ok {
		t.Fatalf("ListMiniMaxTenants(second page) response = %#v", secondResp)
	}
	if len(second.Items) != 1 || second.Items[0].Name == first.Items[0].Name {
		t.Fatalf("ListMiniMaxTenants(second page) = %#v", second)
	}

	invalidBody := adminservice.MiniMaxTenantUpsert{
		Name:           "missing-cred",
		AppId:          "app-x",
		GroupId:        "group-x",
		CredentialName: "not-found",
	}
	invalidResp, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &invalidBody})
	if err != nil {
		t.Fatalf("CreateMiniMaxTenant(missing cred) error = %v", err)
	}
	if _, ok := invalidResp.(adminservice.CreateMiniMaxTenant400JSONResponse); !ok {
		t.Fatalf("CreateMiniMaxTenant(missing cred) response = %#v", invalidResp)
	}

	nilCreateResp, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{})
	if err != nil {
		t.Fatalf("CreateMiniMaxTenant(nil body) error = %v", err)
	}
	if _, ok := nilCreateResp.(adminservice.CreateMiniMaxTenant400JSONResponse); !ok {
		t.Fatalf("CreateMiniMaxTenant(nil body) response = %#v", nilCreateResp)
	}

	nilPutResp, err := srv.PutMiniMaxTenant(ctx, adminservice.PutMiniMaxTenantRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("PutMiniMaxTenant(nil body) error = %v", err)
	}
	if _, ok := nilPutResp.(adminservice.PutMiniMaxTenant400JSONResponse); !ok {
		t.Fatalf("PutMiniMaxTenant(nil body) response = %#v", nilPutResp)
	}

	getMissingResp, err := srv.GetMiniMaxTenant(ctx, adminservice.GetMiniMaxTenantRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("GetMiniMaxTenant(missing) error = %v", err)
	}
	if _, ok := getMissingResp.(adminservice.GetMiniMaxTenant404JSONResponse); !ok {
		t.Fatalf("GetMiniMaxTenant(missing) response = %#v", getMissingResp)
	}

	deleteMissingResp, err := srv.DeleteMiniMaxTenant(ctx, adminservice.DeleteMiniMaxTenantRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("DeleteMiniMaxTenant(missing) error = %v", err)
	}
	if _, ok := deleteMissingResp.(adminservice.DeleteMiniMaxTenant404JSONResponse); !ok {
		t.Fatalf("DeleteMiniMaxTenant(missing) response = %#v", deleteMissingResp)
	}
}

func TestServerVoicesCRUDAndFilters(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	createBody := mustVoiceUpsert(t, `{
		"id": "manual:voice-2",
		"name": "custom",
		"source": "manual",
		"provider": {"kind": "local", "name": "manual"},
		"raw": {"lang": "zh"}
	}`)
	createResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateVoice() error = %v", err)
	}
	created, ok := createResp.(adminservice.CreateVoice200JSONResponse)
	if !ok {
		t.Fatalf("CreateVoice() response = %#v", createResp)
	}
	if created.Id != "manual:voice-2" || created.Source != apitypes.Manual {
		t.Fatalf("CreateVoice() voice = %#v", created)
	}

	listResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{})
	if err != nil {
		t.Fatalf("ListVoices() error = %v", err)
	}
	listed, ok := listResp.(adminservice.ListVoices200JSONResponse)
	if !ok {
		t.Fatalf("ListVoices() response = %#v", listResp)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("ListVoices() items = %#v", listed.Items)
	}

	source := adminservice.VoiceSource(apitypes.Manual)
	filteredResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{
		Params: adminservice.ListVoicesParams{Source: &source},
	})
	if err != nil {
		t.Fatalf("ListVoices(source filter) error = %v", err)
	}
	filtered, ok := filteredResp.(adminservice.ListVoices200JSONResponse)
	if !ok {
		t.Fatalf("ListVoices(source filter) response = %#v", filteredResp)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Id != created.Id {
		t.Fatalf("ListVoices(source filter) = %#v", filtered)
	}

	providerKind := adminservice.VoiceProviderKind("local")
	providerName := adminservice.VoiceProviderName("manual")
	providerResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{
		Params: adminservice.ListVoicesParams{
			ProviderKind: &providerKind,
			ProviderName: &providerName,
		},
	})
	if err != nil {
		t.Fatalf("ListVoices(provider filter) error = %v", err)
	}
	providerListed, ok := providerResp.(adminservice.ListVoices200JSONResponse)
	if !ok {
		t.Fatalf("ListVoices(provider filter) response = %#v", providerResp)
	}
	if len(providerListed.Items) != 1 || providerListed.Items[0].Id != created.Id {
		t.Fatalf("ListVoices(provider filter) = %#v", providerListed)
	}

	getResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: created.Id})
	if err != nil {
		t.Fatalf("GetVoice() error = %v", err)
	}
	got, ok := getResp.(adminservice.GetVoice200JSONResponse)
	if !ok {
		t.Fatalf("GetVoice() response = %#v", getResp)
	}
	if got.Id != created.Id || got.Source != apitypes.Manual {
		t.Fatalf("GetVoice() voice = %#v", got)
	}

	updateBody := mustVoiceUpsert(t, `{
		"id": "manual:voice-2",
		"name": "custom-updated",
		"description": "updated by api",
		"source": "manual",
		"provider": {"kind": "local", "name": "manual"}
	}`)
	putResp, err := srv.PutVoice(ctx, adminservice.PutVoiceRequestObject{
		Id:   "manual:voice-2",
		Body: &updateBody,
	})
	if err != nil {
		t.Fatalf("PutVoice() error = %v", err)
	}
	updated, ok := putResp.(adminservice.PutVoice200JSONResponse)
	if !ok {
		t.Fatalf("PutVoice() response = %#v", putResp)
	}
	if updated.Name == nil || *updated.Name != "custom-updated" {
		t.Fatalf("PutVoice() voice = %#v", updated)
	}

	deleteResp, err := srv.DeleteVoice(ctx, adminservice.DeleteVoiceRequestObject{Id: "manual:voice-2"})
	if err != nil {
		t.Fatalf("DeleteVoice() error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteVoice200JSONResponse); !ok {
		t.Fatalf("DeleteVoice() response = %#v", deleteResp)
	}

	missingResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: "missing"})
	if err != nil {
		t.Fatalf("GetVoice(missing) error = %v", err)
	}
	if _, ok := missingResp.(adminservice.GetVoice404JSONResponse); !ok {
		t.Fatalf("GetVoice(missing) response = %#v", missingResp)
	}
}

func TestServerVoicesPaginationWithEscapedIDs(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	for _, body := range []adminservice.VoiceUpsert{
		mustVoiceUpsert(t, `{
			"id": "manual:voice-1",
			"source": "manual",
			"provider": {"kind": "local", "name": "manual"}
		}`),
		mustVoiceUpsert(t, `{
			"id": "manual:voice-2",
			"source": "manual",
			"provider": {"kind": "local", "name": "manual"}
		}`),
	} {
		if _, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &body}); err != nil {
			t.Fatalf("CreateVoice(%q) error = %v", body.Id, err)
		}
	}

	limit := adminservice.Limit(1)
	firstResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{
		Params: adminservice.ListVoicesParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListVoices(first page) error = %v", err)
	}
	first, ok := firstResp.(adminservice.ListVoices200JSONResponse)
	if !ok {
		t.Fatalf("ListVoices(first page) response = %#v", firstResp)
	}
	if len(first.Items) != 1 || !first.HasNext || first.NextCursor == nil {
		t.Fatalf("ListVoices(first page) = %#v", first)
	}

	cursor := adminservice.Cursor(*first.NextCursor)
	secondResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{
		Params: adminservice.ListVoicesParams{
			Cursor: &cursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListVoices(second page) error = %v", err)
	}
	second, ok := secondResp.(adminservice.ListVoices200JSONResponse)
	if !ok {
		t.Fatalf("ListVoices(second page) response = %#v", secondResp)
	}
	if len(second.Items) != 1 || second.Items[0].Id == first.Items[0].Id {
		t.Fatalf("ListVoices(second page) = %#v", second)
	}
}

func TestServerVoicesRejectSyncWritesButAllowDelete(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	now := srv.now()

	createBody := mustVoiceUpsert(t, `{
		"id": "sync:voice-create",
		"source": "sync",
		"provider": {"kind": "minimax-tenant", "name": "tenant-a"}
	}`)
	createResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateVoice(sync) error = %v", err)
	}
	if _, ok := createResp.(adminservice.CreateVoice400JSONResponse); !ok {
		t.Fatalf("CreateVoice(sync) response = %#v", createResp)
	}

	syncVoice := apitypes.Voice{
		CreatedAt: now,
		Id:        "minimax-tenant:tenant-a:voice-1",
		Name:      stringPtr("narrator"),
		Provider: apitypes.VoiceProvider{
			Kind: miniMaxVoiceProviderKind,
			Name: apitypes.VoiceProviderName("tenant-a"),
		},
		ProviderVoiceId:   stringPtr("voice-1"),
		ProviderVoiceType: stringPtr("system"),
		Source:            apitypes.Sync,
		SyncedAt:          timePtr(now),
		UpdatedAt:         now,
	}
	if err := writeVoice(ctx, srv.Store, syncVoice, nil); err != nil {
		t.Fatalf("writeVoice(sync) error = %v", err)
	}

	putBody := mustVoiceUpsert(t, `{
		"id": "minimax-tenant:tenant-a:voice-1",
		"name": "cannot-update",
		"source": "manual",
		"provider": {"kind": "local", "name": "manual"}
	}`)
	putResp, err := srv.PutVoice(ctx, adminservice.PutVoiceRequestObject{
		Id:   "minimax-tenant:tenant-a:voice-1",
		Body: &putBody,
	})
	if err != nil {
		t.Fatalf("PutVoice(sync existing) error = %v", err)
	}
	if _, ok := putResp.(adminservice.PutVoice409JSONResponse); !ok {
		t.Fatalf("PutVoice(sync existing) response = %#v", putResp)
	}

	deleteResp, err := srv.DeleteVoice(ctx, adminservice.DeleteVoiceRequestObject{Id: syncVoice.Id})
	if err != nil {
		t.Fatalf("DeleteVoice(sync) error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteVoice200JSONResponse); !ok {
		t.Fatalf("DeleteVoice(sync) response = %#v", deleteResp)
	}
	if _, err := getVoice(ctx, srv.Store, string(syncVoice.Id)); err != kv.ErrNotFound {
		t.Fatalf("getVoice(sync after delete) err = %v, want kv.ErrNotFound", err)
	}
}

func TestServerVoiceValidationAndConflictPaths(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	body := mustVoiceUpsert(t, `{
		"id": "manual:voice-1",
		"source": "manual",
		"provider": {"kind": "local", "name": "manual"}
	}`)
	if _, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &body}); err != nil {
		t.Fatalf("CreateVoice(seed) error = %v", err)
	}

	conflictResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("CreateVoice(conflict) error = %v", err)
	}
	if _, ok := conflictResp.(adminservice.CreateVoice409JSONResponse); !ok {
		t.Fatalf("CreateVoice(conflict) response = %#v", conflictResp)
	}

	invalidProvider := mustVoiceUpsert(t, `{
		"id": "manual:voice-2",
		"source": "manual",
		"provider": {"kind": "", "name": "manual"}
	}`)
	invalidProviderResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{Body: &invalidProvider})
	if err != nil {
		t.Fatalf("CreateVoice(invalid provider) error = %v", err)
	}
	if _, ok := invalidProviderResp.(adminservice.CreateVoice400JSONResponse); !ok {
		t.Fatalf("CreateVoice(invalid provider) response = %#v", invalidProviderResp)
	}

	pathMismatch := mustVoiceUpsert(t, `{
		"id": "other-id",
		"source": "manual",
		"provider": {"kind": "local", "name": "manual"}
	}`)
	pathMismatchResp, err := srv.PutVoice(ctx, adminservice.PutVoiceRequestObject{
		Id:   "manual:voice-1",
		Body: &pathMismatch,
	})
	if err != nil {
		t.Fatalf("PutVoice(path mismatch) error = %v", err)
	}
	if _, ok := pathMismatchResp.(adminservice.PutVoice400JSONResponse); !ok {
		t.Fatalf("PutVoice(path mismatch) response = %#v", pathMismatchResp)
	}

	syncBody := mustVoiceUpsert(t, `{
		"id": "manual:voice-3",
		"source": "sync",
		"provider": {"kind": "local", "name": "manual"}
	}`)
	syncResp, err := srv.PutVoice(ctx, adminservice.PutVoiceRequestObject{
		Id:   "manual:voice-3",
		Body: &syncBody,
	})
	if err != nil {
		t.Fatalf("PutVoice(sync body) error = %v", err)
	}
	if _, ok := syncResp.(adminservice.PutVoice400JSONResponse); !ok {
		t.Fatalf("PutVoice(sync body) response = %#v", syncResp)
	}

	deleteMissingResp, err := srv.DeleteVoice(ctx, adminservice.DeleteVoiceRequestObject{Id: "missing"})
	if err != nil {
		t.Fatalf("DeleteVoice(missing) error = %v", err)
	}
	if _, ok := deleteMissingResp.(adminservice.DeleteVoice404JSONResponse); !ok {
		t.Fatalf("DeleteVoice(missing) response = %#v", deleteMissingResp)
	}

	nilCreateResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{})
	if err != nil {
		t.Fatalf("CreateVoice(nil body) error = %v", err)
	}
	if _, ok := nilCreateResp.(adminservice.CreateVoice400JSONResponse); !ok {
		t.Fatalf("CreateVoice(nil body) response = %#v", nilCreateResp)
	}

	nilPutResp, err := srv.PutVoice(ctx, adminservice.PutVoiceRequestObject{Id: "manual:voice-1"})
	if err != nil {
		t.Fatalf("PutVoice(nil body) error = %v", err)
	}
	if _, ok := nilPutResp.(adminservice.PutVoice400JSONResponse); !ok {
		t.Fatalf("PutVoice(nil body) response = %#v", nilPutResp)
	}
}

func TestServerMiniMaxCredentialValidation(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	tenant := apitypes.MiniMaxTenant{
		AppId:          "app-1",
		CredentialName: "cred-main",
		GroupId:        "group-1",
		Name:           "tenant-a",
	}

	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "openai",
		Method:    apitypes.ApiKey,
		Body:      apitypes.CredentialBody{"api_key": "sk-test"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})
	if _, err := srv.miniMaxClientForTenant(ctx, srv.Store, tenant); err == nil {
		t.Fatalf("miniMaxClientForTenant(openai provider) error = nil, want error")
	}

	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.ApiKey,
		Body:      apitypes.CredentialBody{"other": "value"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})
	if _, err := srv.miniMaxClientForTenant(ctx, srv.Store, tenant); err == nil {
		t.Fatalf("miniMaxClientForTenant(missing api key) error = nil, want error")
	}

	missingTenantResp, err := srv.SyncMiniMaxTenantVoices(ctx, adminservice.SyncMiniMaxTenantVoicesRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("SyncMiniMaxTenantVoices(missing tenant) error = %v", err)
	}
	if _, ok := missingTenantResp.(adminservice.SyncMiniMaxTenantVoices404JSONResponse); !ok {
		t.Fatalf("SyncMiniMaxTenantVoices(missing tenant) response = %#v", missingTenantResp)
	}
}

func TestServerMiniMaxHelpers(t *testing.T) {
	t.Parallel()

	if got := miniMaxBaseURL(apitypes.MiniMaxTenant{}); got != defaultMiniMaxBaseURL {
		t.Fatalf("miniMaxBaseURL(default) = %q, want %q", got, defaultMiniMaxBaseURL)
	}
	baseURL := "https://voice.example.test"
	if got := miniMaxBaseURL(apitypes.MiniMaxTenant{BaseUrl: &baseURL}); got != baseURL {
		t.Fatalf("miniMaxBaseURL(tenant) = %q, want %q", got, baseURL)
	}
	if got := credentialBodyString(apitypes.CredentialBody{"api_key": 12345}, "api_key"); got != "12345" {
		t.Fatalf("credentialBodyString(number) = %q, want 12345", got)
	}
	left := map[string]interface{}{"a": float64(1), "b": "text"}
	right := map[string]interface{}{"b": "text", "a": float64(1)}
	if !rawEqual(&left, &right) {
		t.Fatalf("rawEqual() = false, want true")
	}
	different := map[string]interface{}{"a": float64(2)}
	if rawEqual(&left, &different) {
		t.Fatalf("rawEqual(different) = true, want false")
	}
	if rawEqual(nil, &right) || rawEqual(&left, nil) {
		t.Fatalf("rawEqual(nil) = true, want false")
	}
	if matchesVoiceFilters(apitypes.Voice{Source: apitypes.Manual}, voiceFilters{source: stringPtr("sync")}) {
		t.Fatalf("matchesVoiceFilters(source mismatch) = true, want false")
	}
}

func TestServerMiniMaxStoreNotConfigured(t *testing.T) {
	t.Parallel()

	srv := &Server{}
	ctx := context.Background()
	listResp, err := srv.ListMiniMaxTenants(ctx, adminservice.ListMiniMaxTenantsRequestObject{})
	if err != nil {
		t.Fatalf("ListMiniMaxTenants() error = %v", err)
	}
	if _, ok := listResp.(adminservice.ListMiniMaxTenants500JSONResponse); !ok {
		t.Fatalf("ListMiniMaxTenants() response = %#v", listResp)
	}
	voiceResp, err := srv.ListVoices(ctx, adminservice.ListVoicesRequestObject{})
	if err != nil {
		t.Fatalf("ListVoices() error = %v", err)
	}
	if _, ok := voiceResp.(adminservice.ListVoices500JSONResponse); !ok {
		t.Fatalf("ListVoices() response = %#v", voiceResp)
	}
	getVoiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: "missing"})
	if err != nil {
		t.Fatalf("GetVoice() error = %v", err)
	}
	if _, ok := getVoiceResp.(adminservice.GetVoice500JSONResponse); !ok {
		t.Fatalf("GetVoice() response = %#v", getVoiceResp)
	}
	createVoiceResp, err := srv.CreateVoice(ctx, adminservice.CreateVoiceRequestObject{})
	if err != nil {
		t.Fatalf("CreateVoice() error = %v", err)
	}
	if _, ok := createVoiceResp.(adminservice.CreateVoice500JSONResponse); !ok {
		t.Fatalf("CreateVoice() response = %#v", createVoiceResp)
	}
}

func TestServerMiniMaxTenantValidationAndConflictPaths(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.Token,
		Body:      apitypes.CredentialBody{"token": "tok-main"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})

	body := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main"
	}`)
	if _, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &body}); err != nil {
		t.Fatalf("CreateMiniMaxTenant(seed) error = %v", err)
	}

	conflictResp, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("CreateMiniMaxTenant(conflict) error = %v", err)
	}
	if _, ok := conflictResp.(adminservice.CreateMiniMaxTenant409JSONResponse); !ok {
		t.Fatalf("CreateMiniMaxTenant(conflict) response = %#v", conflictResp)
	}

	pathMismatch := mustMiniMaxTenantUpsert(t, `{
		"name": "other-name",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main"
	}`)
	pathMismatchResp, err := srv.PutMiniMaxTenant(ctx, adminservice.PutMiniMaxTenantRequestObject{
		Name: "tenant-a",
		Body: &pathMismatch,
	})
	if err != nil {
		t.Fatalf("PutMiniMaxTenant(path mismatch) error = %v", err)
	}
	if _, ok := pathMismatchResp.(adminservice.PutMiniMaxTenant400JSONResponse); !ok {
		t.Fatalf("PutMiniMaxTenant(path mismatch) response = %#v", pathMismatchResp)
	}

	invalidBaseURL := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-b",
		"app_id": "app-2",
		"group_id": "group-2",
		"credential_name": "cred-main",
		"base_url": "not-a-url"
	}`)
	invalidBaseURLResp, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &invalidBaseURL})
	if err != nil {
		t.Fatalf("CreateMiniMaxTenant(invalid base_url) error = %v", err)
	}
	if _, ok := invalidBaseURLResp.(adminservice.CreateMiniMaxTenant400JSONResponse); !ok {
		t.Fatalf("CreateMiniMaxTenant(invalid base_url) response = %#v", invalidBaseURLResp)
	}

	deleteMissingResp, err := srv.DeleteMiniMaxTenant(ctx, adminservice.DeleteMiniMaxTenantRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("DeleteMiniMaxTenant(missing) error = %v", err)
	}
	if _, ok := deleteMissingResp.(adminservice.DeleteMiniMaxTenant404JSONResponse); !ok {
		t.Fatalf("DeleteMiniMaxTenant(missing) response = %#v", deleteMissingResp)
	}
}

func TestServerSyncMiniMaxTenantVoicesUsesTenantBaseURL(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/v1/get_voice" {
			t.Fatalf("path = %s, want /v1/get_voice", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mmx-key" {
			t.Fatalf("authorization = %q, want Bearer mmx-key", got)
		}
		switch got := r.URL.Query().Get("voice_type"); got {
		case "all", "system", "voice_cloning", "voice_generation":
		default:
			t.Fatalf("query.voice_type = %q, want supported voice type", got)
		}
		_, _ = w.Write([]byte(`{
			"base_resp":{"status_code":0,"status_msg":"ok"},
			"voices":[
				{"voice_id":"voice-1","voice_name":"calm narrator","description":["calm"],"voice_type":"system","gender":"female"},
				{"voice_id":"voice-2","voice_name":"fast narrator","description":["fast"],"voice_type":"voice_cloning","gender":"male"}
			],
			"has_more":false
		}`))
	}))
	defer upstream.Close()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.ApiKey,
		Body:      apitypes.CredentialBody{"api_key": "mmx-key", "base_url": "https://models.example.invalid"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})
	tenantBody := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main"
	}`)
	tenantBody.BaseUrl = stringPtr(upstream.URL)
	if _, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &tenantBody}); err != nil {
		t.Fatalf("CreateMiniMaxTenant() error = %v", err)
	}

	resp, err := srv.SyncMiniMaxTenantVoices(ctx, adminservice.SyncMiniMaxTenantVoicesRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("SyncMiniMaxTenantVoices() error = %v", err)
	}
	syncResp, ok := resp.(adminservice.SyncMiniMaxTenantVoices200JSONResponse)
	if !ok {
		t.Fatalf("SyncMiniMaxTenantVoices() response = %#v", resp)
	}
	if syncResp.CreatedCount != 2 || syncResp.UpdatedCount != 0 || syncResp.DeletedCount != 0 {
		t.Fatalf("SyncMiniMaxTenantVoices() result = %#v", syncResp)
	}
	if callCount.Load() != 4 {
		t.Fatalf("upstream call count = %d, want 4", callCount.Load())
	}

	voiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: "minimax-tenant:tenant-a:voice-1"})
	if err != nil {
		t.Fatalf("GetVoice(sync voice) error = %v", err)
	}
	voice, ok := voiceResp.(adminservice.GetVoice200JSONResponse)
	if !ok {
		t.Fatalf("GetVoice(sync voice) response = %#v", voiceResp)
	}
	if voice.Source != apitypes.Sync || voice.ProviderVoiceId == nil || *voice.ProviderVoiceId != "voice-1" {
		t.Fatalf("stored sync voice = %#v", voice)
	}
	if voice.Raw == nil || (*voice.Raw)["gender"] != "female" {
		t.Fatalf("stored sync voice raw = %#v", voice.Raw)
	}
}

func TestServerSyncMiniMaxTenantVoicesReconcile(t *testing.T) {
	t.Parallel()

	var stage atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch stage.Load() {
		case 0:
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"voices":[
					{"voice_id":"voice-1","voice_name":"first","voice_type":"system"},
					{"voice_id":"voice-2","voice_name":"second","voice_type":"system"}
				],
				"has_more":false
			}`))
		default:
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"voices":[
					{"voice_id":"voice-1","voice_name":"first-updated","voice_type":"system"}
				],
				"has_more":false
			}`))
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.ApiKey,
		Body:      apitypes.CredentialBody{"api_key": "mmx-key", "base_url": "https://models.example.invalid"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})
	tenantBody := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main"
	}`)
	tenantBody.BaseUrl = stringPtr(upstream.URL)
	if _, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &tenantBody}); err != nil {
		t.Fatalf("CreateMiniMaxTenant() error = %v", err)
	}
	manualVoice := apitypes.Voice{
		CreatedAt: srv.now(),
		Id:        "manual:tenant-a:keep",
		Provider: apitypes.VoiceProvider{
			Kind: miniMaxVoiceProviderKind,
			Name: apitypes.VoiceProviderName("tenant-a"),
		},
		Source:    apitypes.Manual,
		UpdatedAt: srv.now(),
	}
	if err := writeVoice(ctx, srv.Store, manualVoice, nil); err != nil {
		t.Fatalf("writeVoice(manual) error = %v", err)
	}

	firstResp, err := srv.SyncMiniMaxTenantVoices(ctx, adminservice.SyncMiniMaxTenantVoicesRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("first SyncMiniMaxTenantVoices() error = %v", err)
	}
	first, ok := firstResp.(adminservice.SyncMiniMaxTenantVoices200JSONResponse)
	if !ok {
		t.Fatalf("first SyncMiniMaxTenantVoices() response = %#v", firstResp)
	}
	if first.CreatedCount != 2 || first.UpdatedCount != 0 || first.DeletedCount != 0 {
		t.Fatalf("first SyncMiniMaxTenantVoices() result = %#v", first)
	}

	stage.Store(1)
	secondResp, err := srv.SyncMiniMaxTenantVoices(ctx, adminservice.SyncMiniMaxTenantVoicesRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("second SyncMiniMaxTenantVoices() error = %v", err)
	}
	second, ok := secondResp.(adminservice.SyncMiniMaxTenantVoices200JSONResponse)
	if !ok {
		t.Fatalf("second SyncMiniMaxTenantVoices() response = %#v", secondResp)
	}
	if second.CreatedCount != 0 || second.UpdatedCount != 1 || second.DeletedCount != 1 {
		t.Fatalf("second SyncMiniMaxTenantVoices() result = %#v", second)
	}

	updatedVoiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: "minimax-tenant:tenant-a:voice-1"})
	if err != nil {
		t.Fatalf("GetVoice(updated sync voice) error = %v", err)
	}
	updatedVoice, ok := updatedVoiceResp.(adminservice.GetVoice200JSONResponse)
	if !ok {
		t.Fatalf("GetVoice(updated sync voice) response = %#v", updatedVoiceResp)
	}
	if updatedVoice.Name == nil || *updatedVoice.Name != "first-updated" {
		t.Fatalf("updated sync voice = %#v", updatedVoice)
	}

	deletedVoiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: "minimax-tenant:tenant-a:voice-2"})
	if err != nil {
		t.Fatalf("GetVoice(deleted sync voice) error = %v", err)
	}
	if _, ok := deletedVoiceResp.(adminservice.GetVoice404JSONResponse); !ok {
		t.Fatalf("GetVoice(deleted sync voice) response = %#v", deletedVoiceResp)
	}

	manualVoiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: manualVoice.Id})
	if err != nil {
		t.Fatalf("GetVoice(manual voice) error = %v", err)
	}
	if _, ok := manualVoiceResp.(adminservice.GetVoice200JSONResponse); !ok {
		t.Fatalf("GetVoice(manual voice) response = %#v", manualVoiceResp)
	}
}

func TestServerSyncMiniMaxTenantVoicesFetchesAllVoiceTypes(t *testing.T) {
	t.Parallel()

	typeCounts := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		voiceType := r.URL.Query().Get("voice_type")
		typeCounts[voiceType]++
		w.Header().Set("Content-Type", "application/json")
		switch voiceType {
		case "all":
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"voices":[
					{"voice_id":"voice-system-1","voice_name":"all-system"}
				],
				"has_more":false
			}`))
		case "system":
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"system_voice":[
					{"voice_id":"voice-system-1","voice_name":"system narrator"}
				],
				"has_more":false
			}`))
		case "voice_cloning":
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"voice_cloning":[
					{"voice_id":"voice-clone-1","voice_name":"clone narrator"}
				],
				"has_more":false
			}`))
		case "voice_generation":
			_, _ = w.Write([]byte(`{
				"base_resp":{"status_code":0,"status_msg":"ok"},
				"voice_generation":[
					{"voice_id":"voice-gen-1","voice_name":"generated narrator"}
				],
				"has_more":false
			}`))
		default:
			t.Fatalf("unexpected voice_type = %q", voiceType)
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t)
	ctx := context.Background()
	seedCredential(t, srv, apitypes.Credential{
		Name:      "cred-main",
		Provider:  "minimax",
		Method:    apitypes.ApiKey,
		Body:      apitypes.CredentialBody{"api_key": "mmx-key", "base_url": "https://models.example.invalid"},
		CreatedAt: srv.now(),
		UpdatedAt: srv.now(),
	})
	tenantBody := mustMiniMaxTenantUpsert(t, `{
		"name": "tenant-a",
		"app_id": "app-1",
		"group_id": "group-1",
		"credential_name": "cred-main"
	}`)
	tenantBody.BaseUrl = stringPtr(upstream.URL)
	if _, err := srv.CreateMiniMaxTenant(ctx, adminservice.CreateMiniMaxTenantRequestObject{Body: &tenantBody}); err != nil {
		t.Fatalf("CreateMiniMaxTenant() error = %v", err)
	}

	resp, err := srv.SyncMiniMaxTenantVoices(ctx, adminservice.SyncMiniMaxTenantVoicesRequestObject{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("SyncMiniMaxTenantVoices() error = %v", err)
	}
	syncResp, ok := resp.(adminservice.SyncMiniMaxTenantVoices200JSONResponse)
	if !ok {
		t.Fatalf("SyncMiniMaxTenantVoices() response = %#v", resp)
	}
	if syncResp.CreatedCount != 3 || syncResp.UpdatedCount != 0 || syncResp.DeletedCount != 0 {
		t.Fatalf("SyncMiniMaxTenantVoices() result = %#v", syncResp)
	}
	if typeCounts["all"] != 1 || typeCounts["system"] != 1 || typeCounts["voice_cloning"] != 1 || typeCounts["voice_generation"] != 1 {
		t.Fatalf("voice type fetch counts = %#v", typeCounts)
	}

	for _, id := range []string{
		"minimax-tenant:tenant-a:voice-system-1",
		"minimax-tenant:tenant-a:voice-clone-1",
		"minimax-tenant:tenant-a:voice-gen-1",
	} {
		voiceResp, err := srv.GetVoice(ctx, adminservice.GetVoiceRequestObject{Id: id})
		if err != nil {
			t.Fatalf("GetVoice(%s) error = %v", id, err)
		}
		if _, ok := voiceResp.(adminservice.GetVoice200JSONResponse); !ok {
			t.Fatalf("GetVoice(%s) response = %#v", id, voiceResp)
		}
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	store, err := kv.NewBadgerInMemory(nil)
	if err != nil {
		t.Fatalf("NewBadgerInMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	fixed := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	return &Server{
		Store: store,
		Now: func() time.Time {
			return fixed
		},
	}
}

func seedCredential(t *testing.T, srv *Server, credential apitypes.Credential) {
	t.Helper()

	data, err := json.Marshal(credential)
	if err != nil {
		t.Fatalf("json.Marshal(credential) error = %v", err)
	}
	if err := srv.Store.Set(context.Background(), credentialKey(string(credential.Name)), data); err != nil {
		t.Fatalf("Store.Set(credential) error = %v", err)
	}
}

func mustMiniMaxTenantUpsert(t *testing.T, raw string) adminservice.MiniMaxTenantUpsert {
	t.Helper()

	var upsert adminservice.MiniMaxTenantUpsert
	if err := json.Unmarshal([]byte(raw), &upsert); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return upsert
}

func mustVoiceUpsert(t *testing.T, raw string) adminservice.VoiceUpsert {
	t.Helper()

	var upsert adminservice.VoiceUpsert
	if err := json.Unmarshal([]byte(raw), &upsert); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return upsert
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
