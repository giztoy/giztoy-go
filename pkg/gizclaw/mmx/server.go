package mmx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	minimax "github.com/giztoy/minimax-go"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

var (
	miniMaxTenantsRoot          = kv.Key{"minimax-tenants", "by-name"}
	voicesRoot                  = kv.Key{"voices", "by-id"}
	voicesBySourceRoot          = kv.Key{"voices", "by-source"}
	voicesByProviderRoot        = kv.Key{"voices", "by-provider"}
	voicesByProviderVoiceIDRoot = kv.Key{"voices", "by-provider-voice-id"}
	credentialsRoot             = kv.Key{"credentials", "by-name"}
)

const (
	defaultListLimit         = 50
	maxListLimit             = 200
	defaultMiniMaxBaseURL    = "https://api.minimax.io"
	miniMaxVoiceProviderKind = apitypes.VoiceProviderKind("minimax-tenant")
)

type Server struct {
	Store      kv.Store
	HTTPClient *http.Client
	Now        func() time.Time
}

type MiniMaxAdminService interface {
	ListMiniMaxTenants(context.Context, adminservice.ListMiniMaxTenantsRequestObject) (adminservice.ListMiniMaxTenantsResponseObject, error)
	CreateMiniMaxTenant(context.Context, adminservice.CreateMiniMaxTenantRequestObject) (adminservice.CreateMiniMaxTenantResponseObject, error)
	DeleteMiniMaxTenant(context.Context, adminservice.DeleteMiniMaxTenantRequestObject) (adminservice.DeleteMiniMaxTenantResponseObject, error)
	GetMiniMaxTenant(context.Context, adminservice.GetMiniMaxTenantRequestObject) (adminservice.GetMiniMaxTenantResponseObject, error)
	PutMiniMaxTenant(context.Context, adminservice.PutMiniMaxTenantRequestObject) (adminservice.PutMiniMaxTenantResponseObject, error)
	SyncMiniMaxTenantVoices(context.Context, adminservice.SyncMiniMaxTenantVoicesRequestObject) (adminservice.SyncMiniMaxTenantVoicesResponseObject, error)
	CreateVoice(context.Context, adminservice.CreateVoiceRequestObject) (adminservice.CreateVoiceResponseObject, error)
	ListVoices(context.Context, adminservice.ListVoicesRequestObject) (adminservice.ListVoicesResponseObject, error)
	DeleteVoice(context.Context, adminservice.DeleteVoiceRequestObject) (adminservice.DeleteVoiceResponseObject, error)
	GetVoice(context.Context, adminservice.GetVoiceRequestObject) (adminservice.GetVoiceResponseObject, error)
	PutVoice(context.Context, adminservice.PutVoiceRequestObject) (adminservice.PutVoiceResponseObject, error)
}

var _ MiniMaxAdminService = (*Server)(nil)

func (s *Server) ListMiniMaxTenants(ctx context.Context, request adminservice.ListMiniMaxTenantsRequestObject) (adminservice.ListMiniMaxTenantsResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.ListMiniMaxTenants500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	items, hasNext, nextCursor, err := listMiniMaxTenantsPage(ctx, store, cursor, limit)
	if err != nil {
		return adminservice.ListMiniMaxTenants500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListMiniMaxTenants200JSONResponse(adminservice.MiniMaxTenantList{
		HasNext:    hasNext,
		Items:      items,
		NextCursor: nextCursor,
	}), nil
}

func (s *Server) CreateMiniMaxTenant(ctx context.Context, request adminservice.CreateMiniMaxTenantRequestObject) (adminservice.CreateMiniMaxTenantResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.CreateMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if request.Body == nil {
		return adminservice.CreateMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", "request body required")), nil
	}
	tenant, err := normalizeMiniMaxTenantUpsert(*request.Body, "")
	if err != nil {
		return adminservice.CreateMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", err.Error())), nil
	}
	if err := validateTenantReferences(ctx, store, tenant); err != nil {
		return adminservice.CreateMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", err.Error())), nil
	}
	if _, err := store.Get(ctx, miniMaxTenantKey(string(tenant.Name))); err == nil {
		return adminservice.CreateMiniMaxTenant409JSONResponse(adminError("MINIMAX_TENANT_ALREADY_EXISTS", fmt.Sprintf("MiniMax tenant %q already exists", tenant.Name))), nil
	} else if !errors.Is(err, kv.ErrNotFound) {
		return adminservice.CreateMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	now := s.now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	if err := writeMiniMaxTenant(ctx, store, tenant); err != nil {
		return adminservice.CreateMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.CreateMiniMaxTenant200JSONResponse(tenant), nil
}

func (s *Server) DeleteMiniMaxTenant(ctx context.Context, request adminservice.DeleteMiniMaxTenantRequestObject) (adminservice.DeleteMiniMaxTenantResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.DeleteMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	tenant, err := getMiniMaxTenant(ctx, store, name)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.DeleteMiniMaxTenant404JSONResponse(adminError("MINIMAX_TENANT_NOT_FOUND", fmt.Sprintf("MiniMax tenant %q not found", name))), nil
		}
		return adminservice.DeleteMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if err := deleteMiniMaxTenantVoices(ctx, store, tenant.Name); err != nil {
		return adminservice.DeleteMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if err := store.Delete(ctx, miniMaxTenantKey(string(tenant.Name))); err != nil {
		return adminservice.DeleteMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.DeleteMiniMaxTenant200JSONResponse(tenant), nil
}

func (s *Server) GetMiniMaxTenant(ctx context.Context, request adminservice.GetMiniMaxTenantRequestObject) (adminservice.GetMiniMaxTenantResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.GetMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	tenant, err := getMiniMaxTenant(ctx, store, name)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.GetMiniMaxTenant404JSONResponse(adminError("MINIMAX_TENANT_NOT_FOUND", fmt.Sprintf("MiniMax tenant %q not found", name))), nil
		}
		return adminservice.GetMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.GetMiniMaxTenant200JSONResponse(tenant), nil
}

func (s *Server) PutMiniMaxTenant(ctx context.Context, request adminservice.PutMiniMaxTenantRequestObject) (adminservice.PutMiniMaxTenantResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.PutMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if request.Body == nil {
		return adminservice.PutMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", "request body required")), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	tenant, err := normalizeMiniMaxTenantUpsert(*request.Body, name)
	if err != nil {
		return adminservice.PutMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", err.Error())), nil
	}
	if err := validateTenantReferences(ctx, store, tenant); err != nil {
		return adminservice.PutMiniMaxTenant400JSONResponse(adminError("INVALID_MINIMAX_TENANT", err.Error())), nil
	}
	previous, err := getMiniMaxTenant(ctx, store, name)
	if err != nil && !errors.Is(err, kv.ErrNotFound) {
		return adminservice.PutMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	now := s.now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	if err == nil {
		tenant.CreatedAt = previous.CreatedAt
		tenant.LastSyncedAt = cloneTime(previous.LastSyncedAt)
	}
	if err := writeMiniMaxTenant(ctx, store, tenant); err != nil {
		return adminservice.PutMiniMaxTenant500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.PutMiniMaxTenant200JSONResponse(tenant), nil
}

func (s *Server) SyncMiniMaxTenantVoices(ctx context.Context, request adminservice.SyncMiniMaxTenantVoicesRequestObject) (adminservice.SyncMiniMaxTenantVoicesResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.SyncMiniMaxTenantVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	tenant, err := getMiniMaxTenant(ctx, store, name)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.SyncMiniMaxTenantVoices404JSONResponse(adminError("MINIMAX_TENANT_NOT_FOUND", fmt.Sprintf("MiniMax tenant %q not found", name))), nil
		}
		return adminservice.SyncMiniMaxTenantVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	client, err := s.miniMaxClientForTenant(ctx, store, tenant)
	if err != nil {
		return adminservice.SyncMiniMaxTenantVoices400JSONResponse(adminError("INVALID_MINIMAX_TENANT", err.Error())), nil
	}
	upstream, err := listAllMiniMaxVoices(ctx, client)
	if err != nil {
		return adminservice.SyncMiniMaxTenantVoices502JSONResponse(adminError("MINIMAX_SYNC_FAILED", err.Error())), nil
	}
	now := s.now()
	createdCount, updatedCount, deletedCount, err := reconcileTenantVoices(ctx, store, tenant, upstream, now)
	if err != nil {
		return adminservice.SyncMiniMaxTenantVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	tenant.LastSyncedAt = &now
	tenant.UpdatedAt = now
	if err := writeMiniMaxTenant(ctx, store, tenant); err != nil {
		return adminservice.SyncMiniMaxTenantVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.SyncMiniMaxTenantVoices200JSONResponse(adminservice.MiniMaxSyncVoicesResult{
		CreatedCount: createdCount,
		DeletedCount: deletedCount,
		SyncedAt:     now,
		TenantName:   tenant.Name,
		UpdatedCount: updatedCount,
	}), nil
}

func (s *Server) CreateVoice(ctx context.Context, request adminservice.CreateVoiceRequestObject) (adminservice.CreateVoiceResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.CreateVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if request.Body == nil {
		return adminservice.CreateVoice400JSONResponse(adminError("INVALID_VOICE", "request body required")), nil
	}
	voice, err := normalizeVoiceUpsert(*request.Body, "")
	if err != nil {
		return adminservice.CreateVoice400JSONResponse(adminError("INVALID_VOICE", err.Error())), nil
	}
	if _, err := store.Get(ctx, voiceKey(string(voice.Id))); err == nil {
		return adminservice.CreateVoice409JSONResponse(adminError("VOICE_ALREADY_EXISTS", fmt.Sprintf("voice %q already exists", voice.Id))), nil
	} else if !errors.Is(err, kv.ErrNotFound) {
		return adminservice.CreateVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	now := s.now()
	voice.CreatedAt = now
	voice.UpdatedAt = now
	if err := writeVoice(ctx, store, voice, nil); err != nil {
		return adminservice.CreateVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.CreateVoice200JSONResponse(voice), nil
}

func (s *Server) ListVoices(ctx context.Context, request adminservice.ListVoicesRequestObject) (adminservice.ListVoicesResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.ListVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	filters := voiceFilters{}
	if request.Params.Source != nil {
		source := strings.TrimSpace(string(*request.Params.Source))
		if source != "" {
			filters.source = &source
		}
	}
	if request.Params.ProviderKind != nil {
		kind := strings.TrimSpace(string(*request.Params.ProviderKind))
		if kind != "" {
			filters.providerKind = &kind
		}
	}
	if request.Params.ProviderName != nil {
		name := strings.TrimSpace(string(*request.Params.ProviderName))
		if name != "" {
			filters.providerName = &name
		}
	}
	items, hasNext, nextCursor, err := listVoicesPage(ctx, store, filters, cursor, limit)
	if err != nil {
		return adminservice.ListVoices500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListVoices200JSONResponse(adminservice.VoiceList{
		HasNext:    hasNext,
		Items:      items,
		NextCursor: nextCursor,
	}), nil
}

func (s *Server) DeleteVoice(ctx context.Context, request adminservice.DeleteVoiceRequestObject) (adminservice.DeleteVoiceResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.DeleteVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	id, err := url.PathUnescape(string(request.Id))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	voice, err := getVoice(ctx, store, id)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.DeleteVoice404JSONResponse(adminError("VOICE_NOT_FOUND", fmt.Sprintf("voice %q not found", id))), nil
		}
		return adminservice.DeleteVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if err := deleteVoice(ctx, store, voice); err != nil {
		return adminservice.DeleteVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.DeleteVoice200JSONResponse(voice), nil
}

func (s *Server) GetVoice(ctx context.Context, request adminservice.GetVoiceRequestObject) (adminservice.GetVoiceResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.GetVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	id, err := url.PathUnescape(string(request.Id))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	voice, err := getVoice(ctx, store, id)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.GetVoice404JSONResponse(adminError("VOICE_NOT_FOUND", fmt.Sprintf("voice %q not found", id))), nil
		}
		return adminservice.GetVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.GetVoice200JSONResponse(voice), nil
}

func (s *Server) PutVoice(ctx context.Context, request adminservice.PutVoiceRequestObject) (adminservice.PutVoiceResponseObject, error) {
	store, err := s.store()
	if err != nil {
		return adminservice.PutVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	if request.Body == nil {
		return adminservice.PutVoice400JSONResponse(adminError("INVALID_VOICE", "request body required")), nil
	}
	id, err := url.PathUnescape(string(request.Id))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	voice, err := normalizeVoiceUpsert(*request.Body, id)
	if err != nil {
		return adminservice.PutVoice400JSONResponse(adminError("INVALID_VOICE", err.Error())), nil
	}
	previous, err := getVoice(ctx, store, id)
	if err != nil && !errors.Is(err, kv.ErrNotFound) {
		return adminservice.PutVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	now := s.now()
	voice.CreatedAt = now
	voice.UpdatedAt = now
	var previousPtr *apitypes.Voice
	if err == nil {
		if previous.Source == apitypes.Sync {
			return adminservice.PutVoice409JSONResponse(adminError("SYNC_VOICE_READ_ONLY", fmt.Sprintf("voice %q has source sync and cannot be modified via API", previous.Id))), nil
		}
		voice.CreatedAt = previous.CreatedAt
		voice.SyncedAt = cloneTime(previous.SyncedAt)
		previousCopy := previous
		previousPtr = &previousCopy
	}
	if err := writeVoice(ctx, store, voice, previousPtr); err != nil {
		return adminservice.PutVoice500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.PutVoice200JSONResponse(voice), nil
}

type voiceFilters struct {
	source       *string
	providerKind *string
	providerName *string
}

func normalizeVoiceUpsert(in adminservice.VoiceUpsert, expectedID string) (apitypes.Voice, error) {
	id := strings.TrimSpace(string(in.Id))
	if id == "" {
		return apitypes.Voice{}, errors.New("id is required")
	}
	if expectedID != "" && id != expectedID {
		return apitypes.Voice{}, fmt.Errorf("id %q must match path id %q", id, expectedID)
	}
	source := apitypes.VoiceSource(strings.TrimSpace(string(in.Source)))
	if source == "" {
		return apitypes.Voice{}, errors.New("source is required")
	}
	if !source.Valid() {
		return apitypes.Voice{}, fmt.Errorf("unsupported source %q", source)
	}
	if source == apitypes.Sync {
		return apitypes.Voice{}, errors.New("voices with source sync cannot be created or updated via API")
	}
	providerKind := strings.TrimSpace(string(in.Provider.Kind))
	if providerKind == "" {
		return apitypes.Voice{}, errors.New("provider.kind is required")
	}
	providerName := strings.TrimSpace(string(in.Provider.Name))
	if providerName == "" {
		return apitypes.Voice{}, errors.New("provider.name is required")
	}
	voice := apitypes.Voice{
		Id: apitypes.VoiceID(id),
		Provider: apitypes.VoiceProvider{
			Kind: apitypes.VoiceProviderKind(providerKind),
			Name: apitypes.VoiceProviderName(providerName),
		},
		Source: source,
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name != "" {
			voice.Name = &name
		}
	}
	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		if description != "" {
			voice.Description = &description
		}
	}
	if in.ProviderVoiceId != nil {
		providerVoiceID := strings.TrimSpace(*in.ProviderVoiceId)
		if providerVoiceID != "" {
			voice.ProviderVoiceId = &providerVoiceID
		}
	}
	if in.ProviderVoiceType != nil {
		providerVoiceType := strings.TrimSpace(*in.ProviderVoiceType)
		if providerVoiceType != "" {
			voice.ProviderVoiceType = &providerVoiceType
		}
	}
	if in.Raw != nil {
		voice.Raw = cloneMap(in.Raw)
	}
	return voice, nil
}

func listMiniMaxTenantsPage(ctx context.Context, store kv.Store, cursor string, limit int) ([]apitypes.MiniMaxTenant, bool, *string, error) {
	entries, err := kv.ListAfter(ctx, store, miniMaxTenantsRoot, cursorAfterKey(miniMaxTenantsRoot, cursor), limit+1)
	if err != nil {
		return nil, false, nil, err
	}
	pageEntries, hasNext, nextCursor := paginateEntries(entries, limit)
	items := make([]apitypes.MiniMaxTenant, 0, len(pageEntries))
	for _, entry := range pageEntries {
		var tenant apitypes.MiniMaxTenant
		if err := json.Unmarshal(entry.Value, &tenant); err != nil {
			return nil, false, nil, fmt.Errorf("mmx: decode tenant list %s: %w", entry.Key.String(), err)
		}
		items = append(items, tenant)
	}
	return items, hasNext, nextCursor, nil
}

func normalizeMiniMaxTenantUpsert(in adminservice.MiniMaxTenantUpsert, expectedName string) (apitypes.MiniMaxTenant, error) {
	name := strings.TrimSpace(string(in.Name))
	if name == "" {
		return apitypes.MiniMaxTenant{}, errors.New("name is required")
	}
	if expectedName != "" && name != expectedName {
		return apitypes.MiniMaxTenant{}, fmt.Errorf("name %q must match path name %q", name, expectedName)
	}
	appID := strings.TrimSpace(string(in.AppId))
	if appID == "" {
		return apitypes.MiniMaxTenant{}, errors.New("app_id is required")
	}
	groupID := strings.TrimSpace(string(in.GroupId))
	if groupID == "" {
		return apitypes.MiniMaxTenant{}, errors.New("group_id is required")
	}
	credentialName := strings.TrimSpace(string(in.CredentialName))
	if credentialName == "" {
		return apitypes.MiniMaxTenant{}, errors.New("credential_name is required")
	}
	tenant := apitypes.MiniMaxTenant{
		AppId:          apitypes.MiniMaxAppID(appID),
		CredentialName: apitypes.CredentialName(credentialName),
		GroupId:        apitypes.MiniMaxGroupID(groupID),
		Name:           apitypes.MiniMaxTenantName(name),
	}
	if in.BaseUrl != nil {
		baseURL := strings.TrimSpace(*in.BaseUrl)
		if baseURL != "" {
			parsed, err := url.Parse(baseURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return apitypes.MiniMaxTenant{}, errors.New("base_url must be an absolute URL")
			}
			tenant.BaseUrl = &baseURL
		}
	}
	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		if description != "" {
			tenant.Description = &description
		}
	}
	return tenant, nil
}

func validateTenantReferences(ctx context.Context, store kv.Store, tenant apitypes.MiniMaxTenant) error {
	if _, err := store.Get(ctx, credentialKey(string(tenant.CredentialName))); err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return fmt.Errorf("credential %q not found", tenant.CredentialName)
		}
		return err
	}
	return nil
}

func writeMiniMaxTenant(ctx context.Context, store kv.Store, tenant apitypes.MiniMaxTenant) error {
	data, err := json.Marshal(tenant)
	if err != nil {
		return fmt.Errorf("mmx: encode tenant %s: %w", tenant.Name, err)
	}
	if err := store.Set(ctx, miniMaxTenantKey(string(tenant.Name)), data); err != nil {
		return fmt.Errorf("mmx: write tenant %s: %w", tenant.Name, err)
	}
	return nil
}

func getMiniMaxTenant(ctx context.Context, store kv.Store, name string) (apitypes.MiniMaxTenant, error) {
	data, err := store.Get(ctx, miniMaxTenantKey(name))
	if err != nil {
		return apitypes.MiniMaxTenant{}, err
	}
	var tenant apitypes.MiniMaxTenant
	if err := json.Unmarshal(data, &tenant); err != nil {
		return apitypes.MiniMaxTenant{}, fmt.Errorf("mmx: decode tenant %s: %w", name, err)
	}
	return tenant, nil
}

func (s *Server) miniMaxClientForTenant(ctx context.Context, store kv.Store, tenant apitypes.MiniMaxTenant) (*minimax.Client, error) {
	credential, err := getCredential(ctx, store, string(tenant.CredentialName))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, fmt.Errorf("credential %q not found", tenant.CredentialName)
		}
		return nil, err
	}
	provider := strings.TrimSpace(string(credential.Provider))
	if provider != "" && provider != "minimax" {
		return nil, fmt.Errorf("credential %q provider must be minimax", tenant.CredentialName)
	}
	apiKey, err := miniMaxAPIKey(credential)
	if err != nil {
		return nil, err
	}
	baseURL := miniMaxBaseURL(tenant)
	client, err := minimax.NewClient(minimax.Config{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: s.HTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("create MiniMax client: %w", err)
	}
	return client, nil
}

func miniMaxBaseURL(tenant apitypes.MiniMaxTenant) string {
	if tenant.BaseUrl != nil {
		baseURL := strings.TrimSpace(*tenant.BaseUrl)
		if baseURL != "" {
			return baseURL
		}
	}
	return defaultMiniMaxBaseURL
}

func miniMaxAPIKey(credential apitypes.Credential) (string, error) {
	for _, key := range []string{"api_key", "token"} {
		if value := credentialBodyString(credential.Body, key); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("credential %q is missing api_key/token", credential.Name)
}

func credentialBodyString(body apitypes.CredentialBody, key string) string {
	if body == nil {
		return ""
	}
	raw, ok := body[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func listAllMiniMaxVoices(ctx context.Context, client *minimax.Client) ([]minimax.Voice, error) {
	// Some MiniMax accounts do not return a fully populated catalog from a single
	// voice_type=all request. Fetch the aggregate view plus each concrete type and
	// merge by voice_id so sync operates on the full upstream catalog.
	voiceTypes := []string{"all", "system", "voice_cloning", "voice_generation"}
	merged := make(map[string]minimax.Voice)
	for _, voiceType := range voiceTypes {
		voices, err := listMiniMaxVoicesByType(ctx, client, voiceType)
		if err != nil {
			return nil, err
		}
		for _, voice := range voices {
			id := strings.TrimSpace(voice.VoiceID)
			if id == "" {
				return nil, errors.New("MiniMax returned voice without voice_id")
			}
			if existing, ok := merged[id]; ok {
				merged[id] = mergeMiniMaxVoice(existing, voice)
				continue
			}
			merged[id] = cloneMiniMaxVoice(voice)
		}
	}
	all := make([]minimax.Voice, 0, len(merged))
	for _, voice := range merged {
		all = append(all, voice)
	}
	sort.Slice(all, func(i, j int) bool {
		return strings.TrimSpace(all[i].VoiceID) < strings.TrimSpace(all[j].VoiceID)
	})
	return all, nil
}

func listMiniMaxVoicesByType(ctx context.Context, client *minimax.Client, voiceType string) ([]minimax.Voice, error) {
	const pageSize = 100
	var (
		all       []minimax.Voice
		pageToken string
	)
	for {
		req := &minimax.ListVoicesRequest{
			PageSize:  intPtr(pageSize),
			PageToken: pageToken,
			VoiceType: voiceType,
		}
		resp, err := client.Voice.ListVoices(ctx, req)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Voices...)
		if !resp.HasMore || strings.TrimSpace(resp.NextPageToken) == "" {
			return all, nil
		}
		pageToken = strings.TrimSpace(resp.NextPageToken)
	}
}

func mergeMiniMaxVoice(existing, candidate minimax.Voice) minimax.Voice {
	merged := cloneMiniMaxVoice(existing)
	if strings.TrimSpace(merged.VoiceID) == "" {
		return cloneMiniMaxVoice(candidate)
	}
	if strings.TrimSpace(merged.VoiceName) == "" {
		merged.VoiceName = strings.TrimSpace(candidate.VoiceName)
	}
	if len(merged.Description) == 0 && len(candidate.Description) > 0 {
		merged.Description = append([]string(nil), candidate.Description...)
	}
	if strings.TrimSpace(merged.CreatedTime) == "" {
		merged.CreatedTime = strings.TrimSpace(candidate.CreatedTime)
	}
	if strings.TrimSpace(merged.VoiceType) == "" {
		merged.VoiceType = strings.TrimSpace(candidate.VoiceType)
	}
	if len(candidate.Raw) == 0 {
		return merged
	}
	if merged.Raw == nil {
		merged.Raw = cloneMiniMaxRaw(candidate.Raw)
		return merged
	}
	for key, value := range candidate.Raw {
		if _, exists := merged.Raw[key]; exists {
			continue
		}
		merged.Raw[key] = append(json.RawMessage(nil), value...)
	}
	return merged
}

func cloneMiniMaxVoice(in minimax.Voice) minimax.Voice {
	out := in
	if in.Description != nil {
		out.Description = append([]string(nil), in.Description...)
	}
	out.Raw = cloneMiniMaxRaw(in.Raw)
	return out
}

func cloneMiniMaxRaw(in map[string]json.RawMessage) map[string]json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = append(json.RawMessage(nil), value...)
	}
	return out
}

func reconcileTenantVoices(ctx context.Context, store kv.Store, tenant apitypes.MiniMaxTenant, upstream []minimax.Voice, now time.Time) (int32, int32, int32, error) {
	existing, err := listProviderVoices(ctx, store, miniMaxVoiceProviderKind, apitypes.VoiceProviderName(tenant.Name))
	if err != nil {
		return 0, 0, 0, err
	}
	existingByProviderVoiceID := make(map[string]apitypes.Voice, len(existing))
	for _, voice := range existing {
		if voice.Source != apitypes.Sync {
			continue
		}
		if voice.ProviderVoiceId == nil || strings.TrimSpace(*voice.ProviderVoiceId) == "" {
			continue
		}
		existingByProviderVoiceID[strings.TrimSpace(*voice.ProviderVoiceId)] = voice
	}

	seen := make(map[string]struct{}, len(upstream))
	var createdCount, updatedCount int32
	for _, upstreamVoice := range upstream {
		providerVoiceID := strings.TrimSpace(upstreamVoice.VoiceID)
		if providerVoiceID == "" {
			return 0, 0, 0, errors.New("MiniMax returned voice without voice_id")
		}
		seen[providerVoiceID] = struct{}{}
		record := voiceFromMiniMax(tenant.Name, upstreamVoice, now)
		if previous, ok := existingByProviderVoiceID[providerVoiceID]; ok {
			record.CreatedAt = previous.CreatedAt
			if voiceSemanticEqual(previous, record) {
				record.UpdatedAt = previous.UpdatedAt
			} else {
				updatedCount++
			}
			previousCopy := previous
			if err := writeVoice(ctx, store, record, &previousCopy); err != nil {
				return 0, 0, 0, err
			}
			continue
		}
		if occupied, err := getVoice(ctx, store, string(record.Id)); err == nil {
			if occupied.Source != apitypes.Sync {
				return 0, 0, 0, fmt.Errorf("voice id %q is occupied by non-sync resource", record.Id)
			}
			previousCopy := occupied
			if err := writeVoice(ctx, store, record, &previousCopy); err != nil {
				return 0, 0, 0, err
			}
			updatedCount++
			continue
		} else if !errors.Is(err, kv.ErrNotFound) {
			return 0, 0, 0, err
		}
		createdCount++
		if err := writeVoice(ctx, store, record, nil); err != nil {
			return 0, 0, 0, err
		}
	}

	var deletedCount int32
	for providerVoiceID, voice := range existingByProviderVoiceID {
		if _, ok := seen[providerVoiceID]; ok {
			continue
		}
		if err := deleteVoice(ctx, store, voice); err != nil {
			return 0, 0, 0, err
		}
		deletedCount++
	}
	return createdCount, updatedCount, deletedCount, nil
}

func listProviderVoices(ctx context.Context, store kv.Store, kind apitypes.VoiceProviderKind, name apitypes.VoiceProviderName) ([]apitypes.Voice, error) {
	prefix := voiceByProviderPrefix(string(kind), string(name))
	items := make([]apitypes.Voice, 0)
	for entry, err := range store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		if len(entry.Key) == 0 {
			continue
		}
		id := entry.Key[len(entry.Key)-1]
		voice, err := getVoice(ctx, store, unescapeStoreSegment(id))
		if err != nil {
			if errors.Is(err, kv.ErrNotFound) {
				continue
			}
			return nil, err
		}
		items = append(items, voice)
	}
	return items, nil
}

func voiceFromMiniMax(tenantName apitypes.MiniMaxTenantName, upstream minimax.Voice, now time.Time) apitypes.Voice {
	providerVoiceID := strings.TrimSpace(upstream.VoiceID)
	voiceID := stableVoiceID(miniMaxVoiceProviderKind, apitypes.VoiceProviderName(tenantName), providerVoiceID)
	description := strings.TrimSpace(strings.Join(upstream.Description, ", "))
	name := strings.TrimSpace(upstream.VoiceName)
	voiceType := strings.TrimSpace(upstream.VoiceType)
	raw := rawMessagesToMap(upstream.Raw)
	syncedAt := now
	voice := apitypes.Voice{
		CreatedAt: now,
		Id:        apitypes.VoiceID(voiceID),
		Provider: apitypes.VoiceProvider{
			Kind: miniMaxVoiceProviderKind,
			Name: apitypes.VoiceProviderName(tenantName),
		},
		ProviderVoiceId: &providerVoiceID,
		Source:          apitypes.Sync,
		SyncedAt:        &syncedAt,
		UpdatedAt:       now,
	}
	if name != "" {
		voice.Name = &name
	}
	if description != "" {
		voice.Description = &description
	}
	if voiceType != "" {
		voice.ProviderVoiceType = &voiceType
	}
	if raw != nil {
		voice.Raw = raw
	}
	return voice
}

func listVoicesPage(ctx context.Context, store kv.Store, filters voiceFilters, cursor string, limit int) ([]apitypes.Voice, bool, *string, error) {
	prefix := voicesRoot
	switch {
	case filters.providerKind != nil && filters.providerName != nil:
		prefix = voiceByProviderPrefix(*filters.providerKind, *filters.providerName)
	case filters.source != nil:
		prefix = voiceBySourcePrefix(*filters.source)
	}
	items := make([]apitypes.Voice, 0, limit+1)
	for entry, err := range store.List(ctx, prefix) {
		if err != nil {
			return nil, false, nil, err
		}
		if len(entry.Key) == 0 {
			continue
		}
		lastSegment := entry.Key[len(entry.Key)-1]
		if cursor != "" && lastSegment <= cursor {
			continue
		}
		var voice apitypes.Voice
		if prefix.String() == voicesRoot.String() {
			if err := json.Unmarshal(entry.Value, &voice); err != nil {
				return nil, false, nil, fmt.Errorf("mmx: decode voice list %s: %w", entry.Key.String(), err)
			}
		} else {
			decodedID := unescapeStoreSegment(lastSegment)
			voice, err = getVoice(ctx, store, decodedID)
			if err != nil {
				if errors.Is(err, kv.ErrNotFound) {
					continue
				}
				return nil, false, nil, err
			}
		}
		if !matchesVoiceFilters(voice, filters) {
			continue
		}
		items = append(items, voice)
		if len(items) >= limit+1 {
			break
		}
	}
	if len(items) == 0 {
		return nil, false, nil, nil
	}
	hasNext := len(items) > limit
	if !hasNext {
		return items, false, nil, nil
	}
	page := items[:limit]
	next := escapeStoreSegment(string(page[len(page)-1].Id))
	return page, true, &next, nil
}

func matchesVoiceFilters(voice apitypes.Voice, filters voiceFilters) bool {
	if filters.source != nil && string(voice.Source) != *filters.source {
		return false
	}
	if filters.providerKind != nil && string(voice.Provider.Kind) != *filters.providerKind {
		return false
	}
	if filters.providerName != nil && string(voice.Provider.Name) != *filters.providerName {
		return false
	}
	return true
}

func writeVoice(ctx context.Context, store kv.Store, voice apitypes.Voice, previous *apitypes.Voice) error {
	data, err := json.Marshal(voice)
	if err != nil {
		return fmt.Errorf("mmx: encode voice %s: %w", voice.Id, err)
	}
	var deletes []kv.Key
	if previous != nil {
		deletes = staleVoiceIndexKeys(*previous, voice)
	}
	if len(deletes) > 0 {
		if err := store.BatchDelete(ctx, deletes); err != nil {
			return fmt.Errorf("mmx: delete stale voice indexes %s: %w", voice.Id, err)
		}
	}
	entries := []kv.Entry{
		{Key: voiceKey(string(voice.Id)), Value: data},
		{Key: voiceBySourceKey(string(voice.Source), string(voice.Id)), Value: []byte{}},
		{Key: voiceByProviderKey(string(voice.Provider.Kind), string(voice.Provider.Name), string(voice.Id)), Value: []byte{}},
	}
	if voice.ProviderVoiceId != nil && strings.TrimSpace(*voice.ProviderVoiceId) != "" {
		entries = append(entries, kv.Entry{
			Key:   voiceByProviderVoiceIDKey(string(voice.Provider.Kind), string(voice.Provider.Name), strings.TrimSpace(*voice.ProviderVoiceId)),
			Value: []byte(string(voice.Id)),
		})
	}
	if err := store.BatchSet(ctx, entries); err != nil {
		return fmt.Errorf("mmx: write voice %s: %w", voice.Id, err)
	}
	return nil
}

func staleVoiceIndexKeys(previous, next apitypes.Voice) []kv.Key {
	var keys []kv.Key
	if previous.Source != next.Source {
		keys = append(keys, voiceBySourceKey(string(previous.Source), string(previous.Id)))
	}
	if previous.Provider.Kind != next.Provider.Kind || previous.Provider.Name != next.Provider.Name {
		keys = append(keys, voiceByProviderKey(string(previous.Provider.Kind), string(previous.Provider.Name), string(previous.Id)))
	}
	if previous.ProviderVoiceId != nil && strings.TrimSpace(*previous.ProviderVoiceId) != "" {
		nextProviderVoiceID := ""
		if next.ProviderVoiceId != nil {
			nextProviderVoiceID = strings.TrimSpace(*next.ProviderVoiceId)
		}
		if previous.Provider.Kind != next.Provider.Kind ||
			previous.Provider.Name != next.Provider.Name ||
			strings.TrimSpace(*previous.ProviderVoiceId) != nextProviderVoiceID {
			keys = append(keys, voiceByProviderVoiceIDKey(
				string(previous.Provider.Kind),
				string(previous.Provider.Name),
				strings.TrimSpace(*previous.ProviderVoiceId),
			))
		}
	}
	return keys
}

func deleteVoice(ctx context.Context, store kv.Store, voice apitypes.Voice) error {
	keys := []kv.Key{
		voiceKey(string(voice.Id)),
		voiceBySourceKey(string(voice.Source), string(voice.Id)),
		voiceByProviderKey(string(voice.Provider.Kind), string(voice.Provider.Name), string(voice.Id)),
	}
	if voice.ProviderVoiceId != nil && strings.TrimSpace(*voice.ProviderVoiceId) != "" {
		keys = append(keys, voiceByProviderVoiceIDKey(
			string(voice.Provider.Kind),
			string(voice.Provider.Name),
			strings.TrimSpace(*voice.ProviderVoiceId),
		))
	}
	if err := store.BatchDelete(ctx, keys); err != nil {
		return fmt.Errorf("mmx: delete voice %s: %w", voice.Id, err)
	}
	return nil
}

func deleteMiniMaxTenantVoices(ctx context.Context, store kv.Store, tenantName apitypes.MiniMaxTenantName) error {
	voices, err := listProviderVoices(ctx, store, miniMaxVoiceProviderKind, apitypes.VoiceProviderName(tenantName))
	if err != nil {
		return err
	}
	for _, voice := range voices {
		if voice.Source != apitypes.Sync {
			continue
		}
		if err := deleteVoice(ctx, store, voice); err != nil {
			return err
		}
	}
	return nil
}

func getVoice(ctx context.Context, store kv.Store, id string) (apitypes.Voice, error) {
	data, err := store.Get(ctx, voiceKey(id))
	if err != nil {
		return apitypes.Voice{}, err
	}
	var voice apitypes.Voice
	if err := json.Unmarshal(data, &voice); err != nil {
		return apitypes.Voice{}, fmt.Errorf("mmx: decode voice %s: %w", id, err)
	}
	return voice, nil
}

func getCredential(ctx context.Context, store kv.Store, name string) (apitypes.Credential, error) {
	data, err := store.Get(ctx, credentialKey(name))
	if err != nil {
		return apitypes.Credential{}, err
	}
	var credential apitypes.Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return apitypes.Credential{}, fmt.Errorf("mmx: decode credential %s: %w", name, err)
	}
	return credential, nil
}

func voiceSemanticEqual(left, right apitypes.Voice) bool {
	return equalStringPtr(left.Description, right.Description) &&
		equalStringPtr(left.Name, right.Name) &&
		left.Provider.Kind == right.Provider.Kind &&
		left.Provider.Name == right.Provider.Name &&
		equalStringPtr(left.ProviderVoiceId, right.ProviderVoiceId) &&
		equalStringPtr(left.ProviderVoiceType, right.ProviderVoiceType) &&
		left.Source == right.Source &&
		rawEqual(left.Raw, right.Raw)
}

func rawEqual(left, right *map[string]interface{}) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func stableVoiceID(kind apitypes.VoiceProviderKind, name apitypes.VoiceProviderName, providerVoiceID string) string {
	return strings.Join([]string{string(kind), string(name), providerVoiceID}, ":")
}

func rawMessagesToMap(raw map[string]json.RawMessage) *map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		var decoded interface{}
		if err := json.Unmarshal(value, &decoded); err != nil {
			out[key] = string(value)
			continue
		}
		out[key] = decoded
	}
	return &out
}

func miniMaxTenantKey(name string) kv.Key {
	return append(append(kv.Key{}, miniMaxTenantsRoot...), escapeStoreSegment(name))
}

func voiceKey(id string) kv.Key {
	return append(append(kv.Key{}, voicesRoot...), escapeStoreSegment(id))
}

func voiceBySourcePrefix(source string) kv.Key {
	return append(append(kv.Key{}, voicesBySourceRoot...), escapeStoreSegment(source))
}

func voiceBySourceKey(source, id string) kv.Key {
	return append(voiceBySourcePrefix(source), escapeStoreSegment(id))
}

func voiceByProviderPrefix(kind, name string) kv.Key {
	prefix := append(append(kv.Key{}, voicesByProviderRoot...), escapeStoreSegment(kind))
	return append(prefix, escapeStoreSegment(name))
}

func voiceByProviderKey(kind, name, id string) kv.Key {
	return append(voiceByProviderPrefix(kind, name), escapeStoreSegment(id))
}

func voiceByProviderVoiceIDKey(kind, name, providerVoiceID string) kv.Key {
	key := append(append(kv.Key{}, voicesByProviderVoiceIDRoot...), escapeStoreSegment(kind))
	key = append(key, escapeStoreSegment(name))
	return append(key, escapeStoreSegment(providerVoiceID))
}

func credentialKey(name string) kv.Key {
	return append(append(kv.Key{}, credentialsRoot...), escapeStoreSegment(name))
}

func escapeStoreSegment(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	return strings.ReplaceAll(value, ":", "%3A")
}

func unescapeStoreSegment(value string) string {
	unescaped, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return unescaped
}

func normalizeListParams(cursor *adminservice.Cursor, limit *adminservice.Limit) (string, int) {
	nextCursor := ""
	if cursor != nil {
		nextCursor = string(*cursor)
	}
	nextLimit := defaultListLimit
	if limit != nil {
		nextLimit = int(*limit)
	}
	if nextLimit <= 0 {
		nextLimit = defaultListLimit
	}
	if nextLimit > maxListLimit {
		nextLimit = maxListLimit
	}
	return nextCursor, nextLimit
}

func cursorAfterKey(prefix kv.Key, cursor string) kv.Key {
	if cursor == "" {
		return nil
	}
	after := append(kv.Key{}, prefix...)
	return append(after, cursor)
}

func paginateEntries(entries []kv.Entry, limit int) ([]kv.Entry, bool, *string) {
	if len(entries) == 0 {
		return nil, false, nil
	}
	hasNext := len(entries) > limit
	if !hasNext {
		return entries, false, nil
	}
	page := entries[:limit]
	if len(page) == 0 || len(page[len(page)-1].Key) == 0 {
		return page, true, nil
	}
	nextCursor := page[len(page)-1].Key[len(page[len(page)-1].Key)-1]
	return page, true, &nextCursor
}

func equalStringPtr(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func cloneTime(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneMap(in *map[string]interface{}) *map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(*in))
	for key, value := range *in {
		out[key] = value
	}
	return &out
}

func intPtr(value int) *int {
	return &value
}

func (s *Server) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) store() (kv.Store, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("MiniMax store not configured")
	}
	return s.Store, nil
}

func adminError(code, message string) apitypes.ErrorResponse {
	return apitypes.ErrorResponse{
		Error: apitypes.ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
}
