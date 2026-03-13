package project

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists projects.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a new Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create inserts a project and initializes its ticket sequence.
func (s *Store) Create(ctx context.Context, p *Project) error {
	packJSON, err := json.Marshal(p.ContextPack)
	if err != nil {
		return err
	}
	status := p.Status
	if status == "" {
		status = "active"
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO projects (id, org_id, name, slug, repo_url, tech_stack, context_pack, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.OrgID, p.Name, p.Slug, p.RepoURL, p.TechStack, packJSON, status, p.CreatedAt)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO ticket_sequences (project_id, next_val) VALUES ($1, 1)`, p.ID)
	return err
}

// GetByID returns a project by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*Project, error) {
	var p Project
	var packJSON []byte
	var techStack []string
	err := s.pool.QueryRow(ctx,
		`SELECT id, org_id, name, slug, COALESCE(repo_url,''), tech_stack, context_pack, status, created_at
		 FROM projects WHERE id = $1`, id).
		Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.RepoURL, &techStack, &packJSON, &p.Status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.TechStack = techStack
	if len(packJSON) > 0 {
		_ = json.Unmarshal(packJSON, &p.ContextPack)
	}
	return &p, nil
}

// ListByOrgID returns projects for an org. statusFilter: "" or "active" = active only, "closed" = closed only, "all" = no filter.
func (s *Store) ListByOrgID(ctx context.Context, orgID string, statusFilter string) ([]Project, error) {
	q := `SELECT id, org_id, name, slug, repo_url, tech_stack, context_pack, status, created_at
		  FROM projects WHERE org_id = $1`
	args := []any{orgID}
	if statusFilter == "" || statusFilter == "active" {
		q += ` AND status = 'active'`
	} else if statusFilter == "closed" {
		q += ` AND status = 'closed'`
	}
	q += ` ORDER BY name`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Project
	for rows.Next() {
		var p Project
		var packJSON []byte
		var techStack []string
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.RepoURL, &techStack, &packJSON, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.TechStack = techStack
		if len(packJSON) > 0 {
			_ = json.Unmarshal(packJSON, &p.ContextPack)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// UpdateContextPack updates only the context_pack JSONB.
func (s *Store) UpdateContextPack(ctx context.Context, projectID string, pack ContextPack) error {
	packJSON, err := json.Marshal(pack)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE projects SET context_pack = $1 WHERE id = $2`, packJSON, projectID)
	return err
}

// UpdateStatus sets project status to "active" or "closed".
func (s *Store) UpdateStatus(ctx context.Context, projectID, status string) error {
	if status != "active" && status != "closed" {
		return ErrInvalidStatus
	}
	res, err := s.pool.Exec(ctx, `UPDATE projects SET status = $1 WHERE id = $2`, status, projectID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// ErrInvalidStatus is returned when status is not "active" or "closed".
var ErrInvalidStatus = errors.New("invalid status: must be active or closed")
// ErrProjectNotFound is returned when UpdateStatus affects no rows.
var ErrProjectNotFound = errors.New("project not found")
