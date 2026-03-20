package workstream

import (
	"context"
	"errors"
	"testing"
)

type mockStore struct {
	createErr  error
	getByID    *WorkStream
	getByIDErr error
	list       []WorkStream
	listErr    error
	updateErr  error

	lastUpdateID, lastUpdateName, lastUpdatePlan, lastUpdateBranch, lastUpdateStatus string
}

func (m *mockStore) Create(ctx context.Context, w *WorkStream) error {
	if m.createErr != nil {
		return m.createErr
	}
	return nil
}

func (m *mockStore) GetByID(ctx context.Context, id string) (*WorkStream, error) {
	return m.getByID, m.getByIDErr
}

func (m *mockStore) ListByProjectID(ctx context.Context, projectID string, statusFilter string) ([]WorkStream, error) {
	return m.list, m.listErr
}

func (m *mockStore) Update(ctx context.Context, id string, name, plan, branch, status string) error {
	m.lastUpdateID, m.lastUpdateName, m.lastUpdatePlan, m.lastUpdateBranch, m.lastUpdateStatus = id, name, plan, branch, status
	return m.updateErr
}

func TestService_CreateWorkStream_Slugify(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)
	ctx := context.Background()

	w, err := svc.CreateWorkStream(ctx, "proj1", "Productionize Feature A", "", "")
	if err != nil {
		t.Fatalf("CreateWorkStream: %v", err)
	}
	if w.Slug != "productionize-feature-a" {
		t.Errorf("Slug: got %q, want productionize-feature-a", w.Slug)
	}
	if w.ProjectID != "proj1" {
		t.Errorf("ProjectID: got %q", w.ProjectID)
	}
	if w.Status != "active" {
		t.Errorf("Status: got %q", w.Status)
	}
	if w.ID == "" {
		t.Error("ID should be set")
	}
}

func TestService_CreateWorkStream_ExplicitSlug(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)
	ctx := context.Background()

	w, err := svc.CreateWorkStream(ctx, "proj1", "Any Name", "custom-slug", "")
	if err != nil {
		t.Fatalf("CreateWorkStream: %v", err)
	}
	if w.Slug != "custom-slug" {
		t.Errorf("Slug: got %q", w.Slug)
	}
}

func TestService_CreateWorkStream_StoreError(t *testing.T) {
	store := &mockStore{createErr: errors.New("db error")}
	svc := NewService(store)
	ctx := context.Background()

	_, err := svc.CreateWorkStream(ctx, "proj1", "X", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_GetWorkStream(t *testing.T) {
	ws := &WorkStream{ID: "ws1", ProjectID: "p1", Name: "Stream", Slug: "stream", Status: "active"}
	store := &mockStore{getByID: ws}
	svc := NewService(store)
	ctx := context.Background()

	got, err := svc.GetWorkStream(ctx, "ws1")
	if err != nil {
		t.Fatalf("GetWorkStream: %v", err)
	}
	if got != ws {
		t.Errorf("got %+v", got)
	}
}

func TestService_ListWorkStreams(t *testing.T) {
	list := []WorkStream{{ID: "ws1", Name: "A", Status: "active"}}
	store := &mockStore{list: list}
	svc := NewService(store)
	ctx := context.Background()

	got, err := svc.ListWorkStreams(ctx, "proj1", "active")
	if err != nil {
		t.Fatalf("ListWorkStreams: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ws1" {
		t.Errorf("got %+v", got)
	}
}

func TestService_UpdateWorkStream_InvalidStatus(t *testing.T) {
	store := &mockStore{getByID: &WorkStream{ID: "ws1", Name: "X", Status: "active"}}
	svc := NewService(store)
	ctx := context.Background()

	err := svc.UpdateWorkStream(ctx, "ws1", "X", "", "", "invalid")
	if err != ErrInvalidStatus {
		t.Errorf("got err %v", err)
	}
}

func TestService_CreateWorkStream_PlanMultiline(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store)
	ctx := context.Background()
	plan := "## Steps\n\n```go\nfunc main() {}\n```\n"
	w, err := svc.CreateWorkStream(ctx, "proj1", "S", "s", plan)
	if err != nil {
		t.Fatalf("CreateWorkStream: %v", err)
	}
	if w.Plan != plan {
		t.Errorf("Plan: got %q want %q", w.Plan, plan)
	}
}

func TestService_UpdateWorkStream_PlanToStore(t *testing.T) {
	store := &mockStore{getByID: &WorkStream{ID: "ws1", Name: "N", Plan: "old", Branch: "b", Status: "active"}}
	svc := NewService(store)
	ctx := context.Background()
	plan := "line1\n\n```mermaid\nflowchart LR\n  A-->B\n```\n"
	if err := svc.UpdateWorkStream(ctx, "ws1", "N", plan, "b", "active"); err != nil {
		t.Fatalf("UpdateWorkStream: %v", err)
	}
	if store.lastUpdatePlan != plan {
		t.Errorf("store plan: got %q want %q", store.lastUpdatePlan, plan)
	}
}
