package workspacetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
	"github.com/gofiber/fiber/v2"
)

func TestServerWorkspaceTemplatesCRUD(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	createDoc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "demo-assistant",
			"description": "single-agent graph workflow"
		},
		"spec": {
			"workspace_layout": {},
			"runtime": {},
			"agents": [],
			"entry_agent": ""
		}
	}`)

	createResp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &createDoc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate() error = %v", err)
	}
	created, ok := createResp.(createWorkspaceTemplate200Response)
	if !ok {
		t.Fatalf("CreateWorkspaceTemplate() response = %#v", createResp)
	}
	if got := discriminatorOf(t, created.doc); got != "SingleAgentGraphWorkflowTemplate" {
		t.Fatalf("CreateWorkspaceTemplate() discriminator = %q", got)
	}

	listResp, err := srv.ListWorkspaceTemplates(ctx, adminservice.ListWorkspaceTemplatesRequestObject{})
	if err != nil {
		t.Fatalf("ListWorkspaceTemplates() error = %v", err)
	}
	listed, ok := listResp.(adminservice.ListWorkspaceTemplates200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaceTemplates() response = %#v", listResp)
	}
	if len(listed.Items) != 1 || listed.HasNext {
		t.Fatalf("ListWorkspaceTemplates() = %#v", listed)
	}

	getResp, err := srv.GetWorkspaceTemplate(ctx, adminservice.GetWorkspaceTemplateRequestObject{Name: "demo-assistant"})
	if err != nil {
		t.Fatalf("GetWorkspaceTemplate() error = %v", err)
	}
	gotDoc, ok := getResp.(getWorkspaceTemplate200Response)
	if !ok {
		t.Fatalf("GetWorkspaceTemplate() response = %#v", getResp)
	}
	gotSingle := mustSingle(t, gotDoc.doc)
	if gotSingle.Metadata.Name != "demo-assistant" {
		t.Fatalf("GetWorkspaceTemplate() name = %q", gotSingle.Metadata.Name)
	}

	updateDoc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "demo-assistant",
			"description": "updated description"
		},
		"spec": {
			"runtime": {
				"executor_ref": "local"
			}
		}
	}`)
	putResp, err := srv.PutWorkspaceTemplate(ctx, adminservice.PutWorkspaceTemplateRequestObject{
		Name: "demo-assistant",
		Body: &updateDoc,
	})
	if err != nil {
		t.Fatalf("PutWorkspaceTemplate() error = %v", err)
	}
	putDoc, ok := putResp.(putWorkspaceTemplate200Response)
	if !ok {
		t.Fatalf("PutWorkspaceTemplate() response = %#v", putResp)
	}
	putSingle := mustSingle(t, putDoc.doc)
	if putSingle.Metadata.Description == nil || *putSingle.Metadata.Description != "updated description" {
		t.Fatalf("PutWorkspaceTemplate() description = %#v", putSingle.Metadata.Description)
	}

	deleteResp, err := srv.DeleteWorkspaceTemplate(ctx, adminservice.DeleteWorkspaceTemplateRequestObject{Name: "demo-assistant"})
	if err != nil {
		t.Fatalf("DeleteWorkspaceTemplate() error = %v", err)
	}
	if _, ok := deleteResp.(deleteWorkspaceTemplate200Response); !ok {
		t.Fatalf("DeleteWorkspaceTemplate() response = %#v", deleteResp)
	}

	getAfterDelete, err := srv.GetWorkspaceTemplate(ctx, adminservice.GetWorkspaceTemplateRequestObject{Name: "demo-assistant"})
	if err != nil {
		t.Fatalf("GetWorkspaceTemplate() after delete error = %v", err)
	}
	if _, ok := getAfterDelete.(adminservice.GetWorkspaceTemplate404JSONResponse); !ok {
		t.Fatalf("GetWorkspaceTemplate() after delete response = %#v", getAfterDelete)
	}
}

func TestServerRejectsUnknownTemplateKind(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "UnknownWorkflowTemplate",
		"metadata": {
			"name": "bad-template"
		},
		"spec": {}
	}`)

	resp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate() error = %v", err)
	}
	if _, ok := resp.(adminservice.CreateWorkspaceTemplate400JSONResponse); !ok {
		t.Fatalf("CreateWorkspaceTemplate() response = %#v", resp)
	}
}

func TestServerAcceptsEmptySingleAgentSpec(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "empty-single"
		},
		"spec": {}
	}`)

	resp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate() error = %v", err)
	}
	created, ok := resp.(createWorkspaceTemplate200Response)
	if !ok {
		t.Fatalf("CreateWorkspaceTemplate() response = %#v", resp)
	}
	single := mustSingle(t, created.doc)
	if single.Metadata.Name != "empty-single" {
		t.Fatalf("CreateWorkspaceTemplate() name = %q", single.Metadata.Name)
	}
}

func TestServerAcceptsEmptyMultiAgentSpec(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "MultiAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "empty-multi"
		},
		"spec": {}
	}`)

	resp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate() error = %v", err)
	}
	created, ok := resp.(createWorkspaceTemplate200Response)
	if !ok {
		t.Fatalf("CreateWorkspaceTemplate() response = %#v", resp)
	}
	multi, err := created.doc.AsMultiAgentGraphWorkflowTemplate()
	if err != nil {
		t.Fatalf("AsMultiAgentGraphWorkflowTemplate() error = %v", err)
	}
	if multi.Metadata.Name != "empty-multi" {
		t.Fatalf("CreateWorkspaceTemplate() name = %q", multi.Metadata.Name)
	}
}

func TestServerPutRejectsPathNameMismatch(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "other-name"
		},
		"spec": {}
	}`)

	resp, err := srv.PutWorkspaceTemplate(ctx, adminservice.PutWorkspaceTemplateRequestObject{
		Name: "expected-name",
		Body: &doc,
	})
	if err != nil {
		t.Fatalf("PutWorkspaceTemplate() error = %v", err)
	}
	if _, ok := resp.(adminservice.PutWorkspaceTemplate400JSONResponse); !ok {
		t.Fatalf("PutWorkspaceTemplate() response = %#v", resp)
	}

	nilCreateResp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate(nil body) error = %v", err)
	}
	if _, ok := nilCreateResp.(adminservice.CreateWorkspaceTemplate400JSONResponse); !ok {
		t.Fatalf("CreateWorkspaceTemplate(nil body) response = %#v", nilCreateResp)
	}

	nilPutResp, err := srv.PutWorkspaceTemplate(ctx, adminservice.PutWorkspaceTemplateRequestObject{Name: "expected-name"})
	if err != nil {
		t.Fatalf("PutWorkspaceTemplate(nil body) error = %v", err)
	}
	if _, ok := nilPutResp.(adminservice.PutWorkspaceTemplate400JSONResponse); !ok {
		t.Fatalf("PutWorkspaceTemplate(nil body) response = %#v", nilPutResp)
	}
}

func TestServerListWorkspaceTemplatesPagination(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		doc := mustDocument(t, fmt.Sprintf(`{
			"apiVersion": "gizclaw.flowcraft/v1alpha1",
			"kind": "SingleAgentGraphWorkflowTemplate",
			"metadata": {
				"name": %q
			},
			"spec": {}
		}`, name))
		if _, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc}); err != nil {
			t.Fatalf("CreateWorkspaceTemplate(%q) error = %v", name, err)
		}
	}

	limit := adminservice.Limit(1)
	firstResp, err := srv.ListWorkspaceTemplates(ctx, adminservice.ListWorkspaceTemplatesRequestObject{
		Params: adminservice.ListWorkspaceTemplatesParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListWorkspaceTemplates(first page) error = %v", err)
	}
	first, ok := firstResp.(adminservice.ListWorkspaceTemplates200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaceTemplates(first page) response = %#v", firstResp)
	}
	if len(first.Items) != 1 || !first.HasNext || first.NextCursor == nil {
		t.Fatalf("ListWorkspaceTemplates(first page) = %#v", first)
	}

	cursor := adminservice.Cursor(*first.NextCursor)
	secondResp, err := srv.ListWorkspaceTemplates(ctx, adminservice.ListWorkspaceTemplatesRequestObject{
		Params: adminservice.ListWorkspaceTemplatesParams{
			Cursor: &cursor,
			Limit:  &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListWorkspaceTemplates(second page) error = %v", err)
	}
	second, ok := secondResp.(adminservice.ListWorkspaceTemplates200JSONResponse)
	if !ok {
		t.Fatalf("ListWorkspaceTemplates(second page) response = %#v", secondResp)
	}
	if len(second.Items) != 1 {
		t.Fatalf("ListWorkspaceTemplates(second page) = %#v", second)
	}
}

func TestServerWorkspaceTemplateConflictAndMissingDelete(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {"name": "duplicate"},
		"spec": {}
	}`)
	if _, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc}); err != nil {
		t.Fatalf("CreateWorkspaceTemplate(seed) error = %v", err)
	}
	duplicateResp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate(duplicate) error = %v", err)
	}
	if _, ok := duplicateResp.(adminservice.CreateWorkspaceTemplate409JSONResponse); !ok {
		t.Fatalf("CreateWorkspaceTemplate(duplicate) response = %#v", duplicateResp)
	}

	deleteResp, err := srv.DeleteWorkspaceTemplate(ctx, adminservice.DeleteWorkspaceTemplateRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("DeleteWorkspaceTemplate(missing) error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteWorkspaceTemplate404JSONResponse); !ok {
		t.Fatalf("DeleteWorkspaceTemplate(missing) response = %#v", deleteResp)
	}
}

func TestServerWorkspaceTemplateStoreNotConfigured(t *testing.T) {
	t.Parallel()

	srv := &Server{}
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {"name": "missing-store"},
		"spec": {}
	}`)

	listResp, err := srv.ListWorkspaceTemplates(ctx, adminservice.ListWorkspaceTemplatesRequestObject{})
	if err != nil {
		t.Fatalf("ListWorkspaceTemplates() error = %v", err)
	}
	if _, ok := listResp.(adminservice.ListWorkspaceTemplates500JSONResponse); !ok {
		t.Fatalf("ListWorkspaceTemplates() response = %#v", listResp)
	}
	createResp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate() error = %v", err)
	}
	if _, ok := createResp.(adminservice.CreateWorkspaceTemplate500JSONResponse); !ok {
		t.Fatalf("CreateWorkspaceTemplate() response = %#v", createResp)
	}
	getResp, err := srv.GetWorkspaceTemplate(ctx, adminservice.GetWorkspaceTemplateRequestObject{Name: "missing-store"})
	if err != nil {
		t.Fatalf("GetWorkspaceTemplate() error = %v", err)
	}
	if _, ok := getResp.(adminservice.GetWorkspaceTemplate500JSONResponse); !ok {
		t.Fatalf("GetWorkspaceTemplate() response = %#v", getResp)
	}
}

func TestServerRejectsMissingTemplateRequiredFields(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	for name, raw := range map[string]string{
		"apiVersion": `{"kind":"SingleAgentGraphWorkflowTemplate","metadata":{"name":"bad"},"spec":{}}`,
		"kind":       `{"apiVersion":"gizclaw.flowcraft/v1alpha1","metadata":{"name":"bad"},"spec":{}}`,
		"name":       `{"apiVersion":"gizclaw.flowcraft/v1alpha1","kind":"SingleAgentGraphWorkflowTemplate","metadata":{},"spec":{}}`,
		"spec":       `{"apiVersion":"gizclaw.flowcraft/v1alpha1","kind":"SingleAgentGraphWorkflowTemplate","metadata":{"name":"bad"}}`,
	} {
		doc := mustDocument(t, raw)
		resp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
		if err != nil {
			t.Fatalf("CreateWorkspaceTemplate(%s) error = %v", name, err)
		}
		if _, ok := resp.(adminservice.CreateWorkspaceTemplate400JSONResponse); !ok {
			t.Fatalf("CreateWorkspaceTemplate(%s) response = %#v", name, resp)
		}
	}
}

func TestServerRejectsUnsupportedTemplateVersion(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()
	doc := mustDocument(t, `{
		"apiVersion": "example.invalid/v1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {"name": "bad-version"},
		"spec": {}
	}`)
	resp, err := srv.CreateWorkspaceTemplate(ctx, adminservice.CreateWorkspaceTemplateRequestObject{Body: &doc})
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate(bad version) error = %v", err)
	}
	if _, ok := resp.(adminservice.CreateWorkspaceTemplate400JSONResponse); !ok {
		t.Fatalf("CreateWorkspaceTemplate(bad version) response = %#v", resp)
	}
}

func TestWorkspaceTemplateResponseVisitors(t *testing.T) {
	t.Parallel()

	doc := mustDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {"name": "visitor"},
		"spec": {}
	}`)
	cases := map[string]func(*fiber.Ctx) error{
		"create": createWorkspaceTemplate200Response{doc: doc}.VisitCreateWorkspaceTemplateResponse,
		"get":    getWorkspaceTemplate200Response{doc: doc}.VisitGetWorkspaceTemplateResponse,
		"put":    putWorkspaceTemplate200Response{doc: doc}.VisitPutWorkspaceTemplateResponse,
		"delete": deleteWorkspaceTemplate200Response{doc: doc}.VisitDeleteWorkspaceTemplateResponse,
	}
	for name, visit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableStartupMessage: true})
			app.Get("/", visit)
			resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("content-type = %q, want application/json", got)
			}
		})
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

func mustDocument(t *testing.T, raw string) apitypes.WorkflowTemplateDocument {
	t.Helper()

	var doc apitypes.WorkflowTemplateDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return doc
}

func discriminatorOf(t *testing.T, doc apitypes.WorkflowTemplateDocument) string {
	t.Helper()

	kind, err := doc.Discriminator()
	if err != nil {
		t.Fatalf("Discriminator() error = %v", err)
	}
	return kind
}

func mustSingle(t *testing.T, doc apitypes.WorkflowTemplateDocument) apitypes.SingleAgentGraphWorkflowTemplate {
	t.Helper()

	single, err := doc.AsSingleAgentGraphWorkflowTemplate()
	if err != nil {
		t.Fatalf("AsSingleAgentGraphWorkflowTemplate() error = %v", err)
	}
	return single
}
