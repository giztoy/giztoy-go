package credential

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestServerCredentialsCRUD(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	createBody := mustCredentialUpsert(t, `{
		"name": "openai-primary",
		"provider": "openai",
		"method": "api_key",
		"description": "primary openai credential",
		"body": {"api_key": "sk-test"}
	}`)
	createResp, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}
	created, ok := createResp.(adminservice.CreateCredential200JSONResponse)
	if !ok {
		t.Fatalf("CreateCredential() response = %#v", createResp)
	}
	if created.Name != "openai-primary" || created.Provider != "openai" || created.Method != apitypes.ApiKey {
		t.Fatalf("CreateCredential() credential = %#v", created)
	}
	if created.Body["api_key"] != "sk-test" {
		t.Fatalf("CreateCredential() body = %#v", created.Body)
	}

	getResp, err := srv.GetCredential(ctx, adminservice.GetCredentialRequestObject{Name: "openai-primary"})
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}
	got, ok := getResp.(adminservice.GetCredential200JSONResponse)
	if !ok {
		t.Fatalf("GetCredential() response = %#v", getResp)
	}
	if got.Description == nil || *got.Description != "primary openai credential" {
		t.Fatalf("GetCredential() description = %#v", got.Description)
	}
	if got.Body["api_key"] != "sk-test" {
		t.Fatalf("GetCredential() body = %#v", got.Body)
	}

	updateBody := mustCredentialUpsert(t, `{
		"name": "openai-primary",
		"provider": "minimax",
		"method": "app_id_token",
		"description": "migrated credential",
		"body": {"app_id": "app-123", "token": "tok-123"}
	}`)
	putResp, err := srv.PutCredential(ctx, adminservice.PutCredentialRequestObject{
		Name: "openai-primary",
		Body: &updateBody,
	})
	if err != nil {
		t.Fatalf("PutCredential() error = %v", err)
	}
	updated, ok := putResp.(adminservice.PutCredential200JSONResponse)
	if !ok {
		t.Fatalf("PutCredential() response = %#v", putResp)
	}
	if updated.Provider != "minimax" || updated.Method != apitypes.AppIdToken {
		t.Fatalf("PutCredential() credential = %#v", updated)
	}
	if updated.Body["app_id"] != "app-123" || updated.Body["token"] != "tok-123" {
		t.Fatalf("PutCredential() body = %#v", updated.Body)
	}

	oldProvider := apitypes.CredentialProvider("openai")
	oldListResp, err := srv.ListCredentials(ctx, adminservice.ListCredentialsRequestObject{
		Params: adminservice.ListCredentialsParams{Provider: &oldProvider},
	})
	if err != nil {
		t.Fatalf("ListCredentials(old provider) error = %v", err)
	}
	oldList, ok := oldListResp.(adminservice.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("ListCredentials(old provider) response = %#v", oldListResp)
	}
	if len(oldList.Items) != 0 {
		t.Fatalf("ListCredentials(old provider) = %#v", oldList)
	}

	newProvider := apitypes.CredentialProvider("minimax")
	newListResp, err := srv.ListCredentials(ctx, adminservice.ListCredentialsRequestObject{
		Params: adminservice.ListCredentialsParams{Provider: &newProvider},
	})
	if err != nil {
		t.Fatalf("ListCredentials(new provider) error = %v", err)
	}
	newList, ok := newListResp.(adminservice.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("ListCredentials(new provider) response = %#v", newListResp)
	}
	if len(newList.Items) != 1 || newList.Items[0].Name != "openai-primary" {
		t.Fatalf("ListCredentials(new provider) = %#v", newList)
	}
	if newList.Items[0].Body["app_id"] != "app-123" {
		t.Fatalf("ListCredentials(new provider) body = %#v", newList.Items[0].Body)
	}

	deleteResp, err := srv.DeleteCredential(ctx, adminservice.DeleteCredentialRequestObject{Name: "openai-primary"})
	if err != nil {
		t.Fatalf("DeleteCredential() error = %v", err)
	}
	if _, ok := deleteResp.(adminservice.DeleteCredential200JSONResponse); !ok {
		t.Fatalf("DeleteCredential() response = %#v", deleteResp)
	}

	getAfterDelete, err := srv.GetCredential(ctx, adminservice.GetCredentialRequestObject{Name: "openai-primary"})
	if err != nil {
		t.Fatalf("GetCredential() after delete error = %v", err)
	}
	if _, ok := getAfterDelete.(adminservice.GetCredential404JSONResponse); !ok {
		t.Fatalf("GetCredential() after delete response = %#v", getAfterDelete)
	}
}

func TestServerListCredentialsPaginationAndFilter(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	for _, raw := range []string{
		`{"name":"alpha","provider":"openai","method":"api_key","body":{"api_key":"a"}}`,
		`{"name":"beta","provider":"openai","method":"api_key","body":{"api_key":"b"}}`,
		`{"name":"gamma","provider":"minimax","method":"api_key","body":{"api_key":"c"}}`,
	} {
		body := mustCredentialUpsert(t, raw)
		if _, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &body}); err != nil {
			t.Fatalf("CreateCredential(%s) error = %v", raw, err)
		}
	}

	limit := adminservice.Limit(1)
	provider := apitypes.CredentialProvider("openai")
	firstResp, err := srv.ListCredentials(ctx, adminservice.ListCredentialsRequestObject{
		Params: adminservice.ListCredentialsParams{
			Provider: &provider,
			Limit:    &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListCredentials(first page) error = %v", err)
	}
	first, ok := firstResp.(adminservice.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("ListCredentials(first page) response = %#v", firstResp)
	}
	if len(first.Items) != 1 || !first.HasNext || first.NextCursor == nil {
		t.Fatalf("ListCredentials(first page) = %#v", first)
	}

	cursor := adminservice.Cursor(*first.NextCursor)
	secondResp, err := srv.ListCredentials(ctx, adminservice.ListCredentialsRequestObject{
		Params: adminservice.ListCredentialsParams{
			Provider: &provider,
			Cursor:   &cursor,
			Limit:    &limit,
		},
	})
	if err != nil {
		t.Fatalf("ListCredentials(second page) error = %v", err)
	}
	second, ok := secondResp.(adminservice.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("ListCredentials(second page) response = %#v", secondResp)
	}
	if len(second.Items) != 1 || second.Items[0].Name == first.Items[0].Name || second.HasNext {
		t.Fatalf("ListCredentials(second page) = %#v", second)
	}
	if _, ok := second.Items[0].Body["api_key"]; !ok {
		t.Fatalf("ListCredentials(second page) body = %#v", second.Items[0].Body)
	}

	allResp, err := srv.ListCredentials(ctx, adminservice.ListCredentialsRequestObject{
		Params: adminservice.ListCredentialsParams{Limit: &limit},
	})
	if err != nil {
		t.Fatalf("ListCredentials(all first page) error = %v", err)
	}
	allFirst, ok := allResp.(adminservice.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("ListCredentials(all first page) response = %#v", allResp)
	}
	if len(allFirst.Items) != 1 || !allFirst.HasNext || allFirst.NextCursor == nil {
		t.Fatalf("ListCredentials(all first page) = %#v", allFirst)
	}
}

func TestServerRejectsMissingBodyOnCreateAndNewPut(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	createBody := mustCredentialUpsert(t, `{
		"name": "alpha",
		"provider": "openai",
		"method": "api_key"
	}`)
	createResp, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &createBody})
	if err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}
	if _, ok := createResp.(adminservice.CreateCredential400JSONResponse); !ok {
		t.Fatalf("CreateCredential() response = %#v", createResp)
	}

	putBody := mustCredentialUpsert(t, `{
		"name": "beta",
		"provider": "openai",
		"method": "api_key"
	}`)
	putResp, err := srv.PutCredential(ctx, adminservice.PutCredentialRequestObject{
		Name: "beta",
		Body: &putBody,
	})
	if err != nil {
		t.Fatalf("PutCredential() error = %v", err)
	}
	if _, ok := putResp.(adminservice.PutCredential400JSONResponse); !ok {
		t.Fatalf("PutCredential() response = %#v", putResp)
	}

	nilCreateResp, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{})
	if err != nil {
		t.Fatalf("CreateCredential(nil body) error = %v", err)
	}
	if _, ok := nilCreateResp.(adminservice.CreateCredential400JSONResponse); !ok {
		t.Fatalf("CreateCredential(nil body) response = %#v", nilCreateResp)
	}

	nilPutResp, err := srv.PutCredential(ctx, adminservice.PutCredentialRequestObject{Name: "beta"})
	if err != nil {
		t.Fatalf("PutCredential(nil body) error = %v", err)
	}
	if _, ok := nilPutResp.(adminservice.PutCredential400JSONResponse); !ok {
		t.Fatalf("PutCredential(nil body) response = %#v", nilPutResp)
	}
}

func TestServerPutRetainsExistingSecretForSameMethod(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	createBody := mustCredentialUpsert(t, `{
		"name": "alpha",
		"provider": "openai",
		"method": "api_key",
		"description": "first",
		"body": {"api_key": "sk-test"}
	}`)
	if _, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &createBody}); err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}

	putBody := mustCredentialUpsert(t, `{
		"name": "alpha",
		"provider": "openai",
		"method": "api_key",
		"description": "second"
	}`)
	putResp, err := srv.PutCredential(ctx, adminservice.PutCredentialRequestObject{
		Name: "alpha",
		Body: &putBody,
	})
	if err != nil {
		t.Fatalf("PutCredential() error = %v", err)
	}
	if _, ok := putResp.(adminservice.PutCredential200JSONResponse); !ok {
		t.Fatalf("PutCredential() response = %#v", putResp)
	}

	record, err := getCredentialRecord(ctx, srv.Store, "alpha")
	if err != nil {
		t.Fatalf("getCredentialRecord() error = %v", err)
	}
	if record.Body["api_key"] != "sk-test" {
		t.Fatalf("stored credential = %#v", record)
	}
	if record.Description == nil || *record.Description != "second" {
		t.Fatalf("stored description = %#v", record.Description)
	}
}

func TestServerPutRejectsPathNameMismatch(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	body := mustCredentialUpsert(t, `{
		"name": "other",
		"provider": "openai",
		"method": "api_key",
		"body": {"api_key": "sk-test"}
	}`)
	resp, err := srv.PutCredential(ctx, adminservice.PutCredentialRequestObject{
		Name: "expected",
		Body: &body,
	})
	if err != nil {
		t.Fatalf("PutCredential() error = %v", err)
	}
	if _, ok := resp.(adminservice.PutCredential400JSONResponse); !ok {
		t.Fatalf("PutCredential() response = %#v", resp)
	}
}

func TestServerCredentialValidationAndMissingPaths(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	ctx := context.Background()

	duplicate := mustCredentialUpsert(t, `{
		"name": "alpha",
		"provider": "openai",
		"method": "api_key",
		"body": {"api_key": "sk-test"}
	}`)
	if _, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &duplicate}); err != nil {
		t.Fatalf("CreateCredential(seed) error = %v", err)
	}
	dupResp, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &duplicate})
	if err != nil {
		t.Fatalf("CreateCredential(duplicate) error = %v", err)
	}
	if _, ok := dupResp.(adminservice.CreateCredential409JSONResponse); !ok {
		t.Fatalf("CreateCredential(duplicate) response = %#v", dupResp)
	}

	badMethod := mustCredentialUpsert(t, `{
		"name": "bad",
		"provider": "openai",
		"method": "unknown",
		"body": {"api_key": "sk-test"}
	}`)
	badResp, err := srv.CreateCredential(ctx, adminservice.CreateCredentialRequestObject{Body: &badMethod})
	if err != nil {
		t.Fatalf("CreateCredential(bad method) error = %v", err)
	}
	if _, ok := badResp.(adminservice.CreateCredential400JSONResponse); !ok {
		t.Fatalf("CreateCredential(bad method) response = %#v", badResp)
	}

	missingDelete, err := srv.DeleteCredential(ctx, adminservice.DeleteCredentialRequestObject{Name: "missing"})
	if err != nil {
		t.Fatalf("DeleteCredential(missing) error = %v", err)
	}
	if _, ok := missingDelete.(adminservice.DeleteCredential404JSONResponse); !ok {
		t.Fatalf("DeleteCredential(missing) response = %#v", missingDelete)
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

func mustCredentialUpsert(t *testing.T, raw string) adminservice.CredentialUpsert {
	t.Helper()

	var upsert adminservice.CredentialUpsert
	if err := json.Unmarshal([]byte(raw), &upsert); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return upsert
}
