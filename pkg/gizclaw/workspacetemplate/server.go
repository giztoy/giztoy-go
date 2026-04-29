package workspacetemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

var templatesRoot = kv.Key{"by-name"}

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

type Server struct {
	Store kv.Store
}

type WorkspaceTemplateAdminService interface {
	ListWorkspaceTemplates(context.Context, adminservice.ListWorkspaceTemplatesRequestObject) (adminservice.ListWorkspaceTemplatesResponseObject, error)
	CreateWorkspaceTemplate(context.Context, adminservice.CreateWorkspaceTemplateRequestObject) (adminservice.CreateWorkspaceTemplateResponseObject, error)
	DeleteWorkspaceTemplate(context.Context, adminservice.DeleteWorkspaceTemplateRequestObject) (adminservice.DeleteWorkspaceTemplateResponseObject, error)
	GetWorkspaceTemplate(context.Context, adminservice.GetWorkspaceTemplateRequestObject) (adminservice.GetWorkspaceTemplateResponseObject, error)
	PutWorkspaceTemplate(context.Context, adminservice.PutWorkspaceTemplateRequestObject) (adminservice.PutWorkspaceTemplateResponseObject, error)
}

var _ WorkspaceTemplateAdminService = (*Server)(nil)

type documentEnvelope struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   templateMetadata `json:"metadata"`
	Spec       *json.RawMessage `json:"spec"`
}

type templateMetadata struct {
	Name string `json:"name"`
}

func (s *Server) ListWorkspaceTemplates(ctx context.Context, request adminservice.ListWorkspaceTemplatesRequestObject) (adminservice.ListWorkspaceTemplatesResponseObject, error) {
	if s == nil || s.Store == nil {
		return adminservice.ListWorkspaceTemplates500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", "workspace template store not configured")), nil
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	entries, err := kv.ListAfter(ctx, s.Store, templatesRoot, cursorAfterKey(templatesRoot, cursor), limit+1)
	if err != nil {
		return adminservice.ListWorkspaceTemplates500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	pageEntries, hasNext, nextCursor := paginateEntries(entries, limit)
	items := make([]apitypes.WorkflowTemplateDocument, 0)
	for _, entry := range pageEntries {
		doc, err := decodeDocument(entry.Value)
		if err != nil {
			return adminservice.ListWorkspaceTemplates500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
		}
		items = append(items, doc)
	}
	return adminservice.ListWorkspaceTemplates200JSONResponse(adminservice.WorkspaceTemplateList{
		HasNext:    hasNext,
		Items:      items,
		NextCursor: nextCursor,
	}), nil
}

func (s *Server) CreateWorkspaceTemplate(ctx context.Context, request adminservice.CreateWorkspaceTemplateRequestObject) (adminservice.CreateWorkspaceTemplateResponseObject, error) {
	if s == nil || s.Store == nil {
		return adminservice.CreateWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", "workspace template store not configured")), nil
	}
	if request.Body == nil {
		return adminservice.CreateWorkspaceTemplate400JSONResponse(apitypes.NewErrorResponse("INVALID_TEMPLATE", "request body required")), nil
	}
	doc, env, raw, err := validateDocument(*request.Body, "")
	if err != nil {
		return adminservice.CreateWorkspaceTemplate400JSONResponse(apitypes.NewErrorResponse("INVALID_TEMPLATE", err.Error())), nil
	}
	key := templateKey(env.Metadata.Name)
	if _, err := s.Store.Get(ctx, key); err == nil {
		return adminservice.CreateWorkspaceTemplate409JSONResponse(apitypes.NewErrorResponse("WORKSPACE_TEMPLATE_ALREADY_EXISTS", fmt.Sprintf("workspace template %q already exists", env.Metadata.Name))), nil
	} else if !errors.Is(err, kv.ErrNotFound) {
		return adminservice.CreateWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	if err := s.Store.Set(ctx, key, raw); err != nil {
		return adminservice.CreateWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	return createWorkspaceTemplate200Response{doc: doc}, nil
}

func (s *Server) DeleteWorkspaceTemplate(ctx context.Context, request adminservice.DeleteWorkspaceTemplateRequestObject) (adminservice.DeleteWorkspaceTemplateResponseObject, error) {
	if s == nil || s.Store == nil {
		return adminservice.DeleteWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", "workspace template store not configured")), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	key := templateKey(name)
	data, err := s.Store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.DeleteWorkspaceTemplate404JSONResponse(apitypes.NewErrorResponse("WORKSPACE_TEMPLATE_NOT_FOUND", fmt.Sprintf("workspace template %q not found", name))), nil
		}
		return adminservice.DeleteWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	doc, err := decodeDocument(data)
	if err != nil {
		return adminservice.DeleteWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	if err := s.Store.Delete(ctx, key); err != nil {
		return adminservice.DeleteWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	return deleteWorkspaceTemplate200Response{doc: doc}, nil
}

func (s *Server) GetWorkspaceTemplate(ctx context.Context, request adminservice.GetWorkspaceTemplateRequestObject) (adminservice.GetWorkspaceTemplateResponseObject, error) {
	if s == nil || s.Store == nil {
		return adminservice.GetWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", "workspace template store not configured")), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	data, err := s.Store.Get(ctx, templateKey(name))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return adminservice.GetWorkspaceTemplate404JSONResponse(apitypes.NewErrorResponse("WORKSPACE_TEMPLATE_NOT_FOUND", fmt.Sprintf("workspace template %q not found", name))), nil
		}
		return adminservice.GetWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	doc, err := decodeDocument(data)
	if err != nil {
		return adminservice.GetWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	return getWorkspaceTemplate200Response{doc: doc}, nil
}

func (s *Server) PutWorkspaceTemplate(ctx context.Context, request adminservice.PutWorkspaceTemplateRequestObject) (adminservice.PutWorkspaceTemplateResponseObject, error) {
	if s == nil || s.Store == nil {
		return adminservice.PutWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", "workspace template store not configured")), nil
	}
	if request.Body == nil {
		return adminservice.PutWorkspaceTemplate400JSONResponse(apitypes.NewErrorResponse("INVALID_TEMPLATE", "request body required")), nil
	}
	name, err := url.PathUnescape(string(request.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	doc, env, raw, err := validateDocument(*request.Body, name)
	if err != nil {
		return adminservice.PutWorkspaceTemplate400JSONResponse(apitypes.NewErrorResponse("INVALID_TEMPLATE", err.Error())), nil
	}
	if err := s.Store.Set(ctx, templateKey(env.Metadata.Name), raw); err != nil {
		return adminservice.PutWorkspaceTemplate500JSONResponse(apitypes.NewErrorResponse("INTERNAL_ERROR", err.Error())), nil
	}
	return putWorkspaceTemplate200Response{doc: doc}, nil
}

func validateDocument(doc apitypes.WorkflowTemplateDocument, expectedName string) (apitypes.WorkflowTemplateDocument, documentEnvelope, []byte, error) {
	var env documentEnvelope
	raw, err := json.Marshal(doc)
	if err != nil {
		return apitypes.WorkflowTemplateDocument{}, env, nil, err
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return apitypes.WorkflowTemplateDocument{}, env, nil, err
	}
	env.APIVersion = strings.TrimSpace(env.APIVersion)
	env.Kind = strings.TrimSpace(env.Kind)
	env.Metadata.Name = strings.TrimSpace(env.Metadata.Name)
	if env.APIVersion == "" {
		return apitypes.WorkflowTemplateDocument{}, env, nil, errors.New("apiVersion is required")
	}
	if env.Kind == "" {
		return apitypes.WorkflowTemplateDocument{}, env, nil, errors.New("kind is required")
	}
	if env.Metadata.Name == "" {
		return apitypes.WorkflowTemplateDocument{}, env, nil, errors.New("metadata.name is required")
	}
	if env.Spec == nil || bytes.Equal(bytes.TrimSpace(*env.Spec), []byte("null")) {
		return apitypes.WorkflowTemplateDocument{}, env, nil, errors.New("spec is required")
	}
	if expectedName != "" && env.Metadata.Name != expectedName {
		return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("metadata.name %q must match path name %q", env.Metadata.Name, expectedName)
	}

	var normalized apitypes.WorkflowTemplateDocument
	switch env.Kind {
	case string(apitypes.SingleAgentGraphWorkflowTemplateKindSingleAgentGraphWorkflowTemplate):
		typed, err := doc.AsSingleAgentGraphWorkflowTemplate()
		if err != nil {
			return apitypes.WorkflowTemplateDocument{}, env, nil, err
		}
		if !typed.ApiVersion.Valid() {
			return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("unsupported apiVersion %q", env.APIVersion)
		}
		if !typed.Kind.Valid() {
			return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("unsupported kind %q", env.Kind)
		}
		if err := normalized.FromSingleAgentGraphWorkflowTemplate(typed); err != nil {
			return apitypes.WorkflowTemplateDocument{}, env, nil, err
		}
	case string(apitypes.MultiAgentGraphWorkflowTemplateKindMultiAgentGraphWorkflowTemplate):
		typed, err := doc.AsMultiAgentGraphWorkflowTemplate()
		if err != nil {
			return apitypes.WorkflowTemplateDocument{}, env, nil, err
		}
		if !typed.ApiVersion.Valid() {
			return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("unsupported apiVersion %q", env.APIVersion)
		}
		if !typed.Kind.Valid() {
			return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("unsupported kind %q", env.Kind)
		}
		if err := normalized.FromMultiAgentGraphWorkflowTemplate(typed); err != nil {
			return apitypes.WorkflowTemplateDocument{}, env, nil, err
		}
	default:
		return apitypes.WorkflowTemplateDocument{}, env, nil, fmt.Errorf("unsupported kind %q", env.Kind)
	}

	raw, err = json.Marshal(normalized)
	if err != nil {
		return apitypes.WorkflowTemplateDocument{}, env, nil, err
	}
	return normalized, env, raw, nil
}

func decodeDocument(data []byte) (apitypes.WorkflowTemplateDocument, error) {
	var doc apitypes.WorkflowTemplateDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return apitypes.WorkflowTemplateDocument{}, err
	}
	return doc, nil
}

func templateKey(name string) kv.Key {
	return append(append(kv.Key{}, templatesRoot...), escapeStoreSegment(name))
}

func escapeStoreSegment(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	return strings.ReplaceAll(value, ":", "%3A")
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
