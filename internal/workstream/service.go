package workstream

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WorkStreamStore is the persistence interface used by Service.
type WorkStreamStore interface {
	Create(ctx context.Context, w *WorkStream) error
	GetByID(ctx context.Context, id string) (*WorkStream, error)
	ListByProjectID(ctx context.Context, projectID string, statusFilter string) ([]WorkStream, error)
	Update(ctx context.Context, id string, name, plan, branch, status string) error
}

// Service provides work stream operations.
type Service struct {
	store WorkStreamStore
}

// NewService returns a new Service.
func NewService(store WorkStreamStore) *Service {
	return &Service{store: store}
}

// CreateWorkStream creates a work stream under a project.
func (s *Service) CreateWorkStream(ctx context.Context, projectID, name, slug, plan string) (*WorkStream, error) {
	if slug == "" {
		slug = slugify(name)
	}
	id := uuid.Must(uuid.NewV7()).String()
	w := &WorkStream{
		ID:          id,
		ProjectID:   projectID,
		Name:        name,
		Slug:        slug,
		Plan:        plan,
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.store.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// GetWorkStream returns a work stream by ID.
func (s *Service) GetWorkStream(ctx context.Context, id string) (*WorkStream, error) {
	return s.store.GetByID(ctx, id)
}

// ListWorkStreams returns work streams for a project. statusFilter: "" or "active" = active only, "closed" = closed only, "all" = all.
func (s *Service) ListWorkStreams(ctx context.Context, projectID string, statusFilter string) ([]WorkStream, error) {
	return s.store.ListByProjectID(ctx, projectID, statusFilter)
}

// UpdateWorkStream updates a work stream (name, plan, branch, status).
// Caller passes values to persist; REST/MCP layers merge with existing fields for partial updates.
func (s *Service) UpdateWorkStream(ctx context.Context, id string, name, plan, branch, status string) error {
	if status != "" && status != "active" && status != "closed" {
		return ErrInvalidStatus
	}
	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if name == "" {
		name = existing.Name
	}
	if status == "" {
		status = existing.Status
	}
	return s.store.Update(ctx, id, name, plan, branch, status)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "work-stream"
	}
	return s
}
