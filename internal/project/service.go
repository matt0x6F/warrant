package project

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service provides project operations.
type Service struct {
	store *Store
}

// NewService returns a new Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateProject creates a project under an org.
func (s *Service) CreateProject(ctx context.Context, orgID, name, slug, repoURL string, techStack []string) (*Project, error) {
	if slug == "" {
		slug = slugify(name)
	}
	id := uuid.Must(uuid.NewV7()).String()
	p := &Project{
		ID:          id,
		OrgID:       orgID,
		Name:        name,
		Slug:        slug,
		RepoURL:     repoURL,
		TechStack:   techStack,
		ContextPack: ContextPack{},
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.store.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateContextPack updates the project's context pack.
func (s *Service) UpdateContextPack(ctx context.Context, projectID string, pack ContextPack) error {
	return s.store.UpdateContextPack(ctx, projectID, pack)
}

// GetProject returns a project by ID.
func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
	return s.store.GetByID(ctx, id)
}

// ListByOrgID returns projects for an org. statusFilter: "" or "active" = active only, "closed" = closed only, "all" = all.
func (s *Service) ListByOrgID(ctx context.Context, orgID string, statusFilter string) ([]Project, error) {
	return s.store.ListByOrgID(ctx, orgID, statusFilter)
}

// UpdateStatus sets project status to "active" or "closed". Returns errProjectNotFound if project does not exist.
func (s *Service) UpdateStatus(ctx context.Context, projectID, status string) error {
	if status != "active" && status != "closed" {
		return ErrInvalidStatus
	}
	return s.store.UpdateStatus(ctx, projectID, status)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "project"
	}
	return s
}
