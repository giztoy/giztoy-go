package workspace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestServerWorkspacesCRUD(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedTemplate(t, srv, "template-1")

	createBody := mustWorkspaceUpsert(t, `{
		"name": "alpha",
		"workspace_template_name": "template-1",
		"parameters": {"mode": "demo"}
	}`)

	createResp, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	created, ok := createResp.(adminservice.CreateWorkspace200JSONResponse)
	if !ok {
		t.Fatalf("CreateWorkspace() response = %#v", createResp)
	}
	if created.Name != "alpha" || created.WorkspaceTemplateName != "template-1" {
		t.Fatalf("CreateWorkspace() workspace = %#v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("CreateWorkspace() timestamps = %#v", created)
	}

	listResp, err := srv.ListWorkspaces(ctx, adminservice.ListWorkspacesRequestObject{})
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	listed, ok := listResp.(adminservice.ListWorkspaces200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaces() response = %#v", listResp)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "alpha" || listed.HasNext {
		t.Fatalf("ListWorkspaces() = %#v", listed)
	}

	getResp, err := srv.GetWorkspace(ctx, adminservice.GetWorkspaceRequestObject{Name: "alpha"})
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	got, ok := getResp.(adminservice.GetWorkspace200JSONResponse)
	if !ok {
		t.Fatalf("GetWorkspace() response = %#v", getResp)
	}
	if got.Name != "alpha" {
		t.Fatalf("GetWorkspace() = %#v", got)
	}

	updateBody := mustWorkspaceUpsert(t, `{
		"name": "alpha",
		"workspace_template_name": "template-1",
		"parameters": {"mode": "updated"}
	}`)
	putResp, err := srv.PutWorkspace(ctx, adminservice.PutWorkspaceRequestObject{
		Name: "alpha",
		Body: &updateBody,
	})
	if err != nil {
		t.Fatalf("PutWorkspace() error = %v", err)
	}
	updated, ok := putResp.(adminservice.PutWorkspace200JSONResponse)
	if !ok {
		t.Fatalf("PutWorkspace() response = %#v", putResp)
	}
	if updated.CreatedAt.IsZero() || updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Fatalf("PutWorkspace() timestamps = %#v", updated)
	}

	deleteResp, err := srv.DeleteWorkspace(ctx, adminservice.DeleteWorkspaceRequestObject{Name: "alpha"})
	if err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteWorkspace200JSONResponse); !ok {
		t.Fatalf("DeleteWorkspace() response = %#v", deleteResp)
	}

	getAfterDelete, err := srv.GetWorkspace(ctx, adminservice.GetWorkspaceRequestObject{Name: "alpha"})
	if err != nil {
		t.Fatalf("GetWorkspace() after delete error = %v", err)
	}
	if _, ok := getAfterDelete.(adminservice.GetWorkspace404JSONResponse); !ok {
		t.Fatalf("GetWorkspace() after delete response = %#v", getAfterDelete)
	}
}

func TestServerListWorkspacesPagination(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedTemplate(t, srv, "template-1")

	for _, name := range []string{"alpha", "beta", "gamma"} {
		body := adminservice.WorkspaceUpsert{
			Name:                  apitypes.WorkspaceName(name),
			WorkspaceTemplateName: "template-1",
		}
		if _, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &body}); err != nil {
			t.Fatalf("CreateWorkspace(%q) error = %v", name, err)
		}
	}

	limit := adminservice.Limit(1)
	firstResp, err := srv.ListWorkspaces(ctx, adminservice.ListWorkspacesRequestObject{
		Params: adminservice.ListWorkspacesParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListWorkspaces(first page) error = %v", err)
	}
	first, ok := firstResp.(adminservice.ListWorkspaces200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaces(first page) response = %#v", firstResp)
	}
	if len(first.Items) != 1 || !first.HasNext || first.NextCursor == nil {
		t.Fatalf("ListWorkspaces(first page) = %#v", first)
	}

	cursor := adminservice.Cursor(*first.NextCursor)
	secondResp, err := srv.ListWorkspaces(ctx, adminservice.ListWorkspacesRequestObject{
		Params: adminservice.ListWorkspacesParams{
			Cursor: &cursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListWorkspaces(second page) error = %v", err)
	}
	second, ok := secondResp.(adminservice.ListWorkspaces200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaces(second page) response = %#v", secondResp)
	}
	if len(second.Items) != 1 || second.Items[0].Name == first.Items[0].Name {
		t.Fatalf("ListWorkspaces(second page) = %#v", second)
	}
}

func TestServerRejectsInvalidWorkspaceReferences(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedTemplate(t, srv, "template-1")

	missingTemplate := mustWorkspaceUpsert(t, `{
		"name": "alpha",
		"workspace_template_name": "missing-template"
	}`)
	resp, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &missingTemplate})
	if err != nil {
		t.Fatalf("CreateWorkspace(missing template) error = %v", err)
	}
	if _, ok := resp.(adminservice.CreateWorkspace400JSONResponse); !ok {
		t.Fatalf("CreateWorkspace(missing template) response = %#v", resp)
	}

	nilCreateResp, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{})
	if err != nil {
		t.Fatalf("CreateWorkspace(nil body) error = %v", err)
	}
	if _, ok := nilCreateResp.(adminservice.CreateWorkspace400JSONResponse); !ok {
		t.Fatalf("CreateWorkspace(nil body) response = %#v", nilCreateResp)
	}

	blankName := mustWorkspaceUpsert(t, `{
		"name": " ",
		"workspace_template_name": "template-1"
	}`)
	blankResp, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &blankName})
	if err != nil {
		t.Fatalf("CreateWorkspace(blank name) error = %v", err)
	}
	if _, ok := blankResp.(adminservice.CreateWorkspace400JSONResponse); !ok {
		t.Fatalf("CreateWorkspace(blank name) response = %#v", blankResp)
	}
}

func TestServerPutRejectsPathNameMismatch(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedTemplate(t, srv, "template-1")

	body := mustWorkspaceUpsert(t, `{
		"name": "other",
		"workspace_template_name": "template-1"
	}`)
	resp, err := srv.PutWorkspace(ctx, adminservice.PutWorkspaceRequestObject{
		Name: "expected",
		Body: &body,
	})
	if err != nil {
		t.Fatalf("PutWorkspace() error = %v", err)
	}
	if _, ok := resp.(adminservice.PutWorkspace400JSONResponse); !ok {
		t.Fatalf("PutWorkspace() response = %#v", resp)
	}

	nilPutResp, err := srv.PutWorkspace(ctx, adminservice.PutWorkspaceRequestObject{Name: "expected"})
	if err != nil {
		t.Fatalf("PutWorkspace(nil body) error = %v", err)
	}
	if _, ok := nilPutResp.(adminservice.PutWorkspace400JSONResponse); !ok {
		t.Fatalf("PutWorkspace(nil body) response = %#v", nilPutResp)
	}
}

func TestServerWorkspaceConflictAndMissingDelete(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	seedTemplate(t, srv, "template-1")

	body := mustWorkspaceUpsert(t, `{
		"name": "alpha",
		"workspace_template_name": "template-1"
	}`)
	if _, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &body}); err != nil {
		t.Fatalf("CreateWorkspace(seed) error = %v", err)
	}
	duplicateResp, err := srv.CreateWorkspace(ctx, adminservice.CreateWorkspaceRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("CreateWorkspace(duplicate) error = %v", err)
	}
	if _, ok := duplicateResp.(adminservice.CreateWorkspace409JSONResponse); !ok {
		t.Fatalf("CreateWorkspace(duplicate) response = %#v", duplicateResp)
	}

	deleteResp, err := srv.DeleteWorkspace(ctx, adminservice.DeleteWorkspaceRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("DeleteWorkspace(missing) error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteWorkspace404JSONResponse); !ok {
		t.Fatalf("DeleteWorkspace(missing) response = %#v", deleteResp)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	store, err := kv.NewBadgerInMemory(nil)
	if err != nil {
		t.Fatalf("NewBadgerInMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &Server{Store: store}
}

func seedTemplate(t *testing.T, srv *Server, name string) {
	t.Helper()

	if err := srv.Store.Set(context.Background(), templateReferenceKey(name), []byte(`{}`)); err != nil {
		t.Fatalf("seed template %q: %v", name, err)
	}
}

func mustWorkspaceUpsert(t *testing.T, raw string) adminservice.WorkspaceUpsert {
	t.Helper()

	var upsert adminservice.WorkspaceUpsert
	if err := json.Unmarshal([]byte(raw), &upsert); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return upsert
}
