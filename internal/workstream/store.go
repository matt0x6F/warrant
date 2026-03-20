package workstream

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrWorkStreamNotFound = errors.New("work stream not found")
	ErrInvalidStatus      = errors.New("invalid status: must be active or closed")
)


// Store persists work streams. Implements WorkStreamStore.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a new Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create inserts a work stream.
func (s *Store) Create(ctx context.Context, w *WorkStream) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO work_streams (id, project_id, name, slug, plan, branch, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		w.ID, w.ProjectID, w.Name, w.Slug, nullIfEmpty(w.Plan), nullIfEmpty(w.Branch), w.Status, w.CreatedAt)
	return err
}

// GetByID returns a work stream by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*WorkStream, error) {
	var w WorkStream
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, name, slug, COALESCE(plan,''), COALESCE(branch,''), status, created_at
		 FROM work_streams WHERE id = $1`, id).
		Scan(&w.ID, &w.ProjectID, &w.Name, &w.Slug, &w.Plan, &w.Branch, &w.Status, &w.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkStreamNotFound
		}
		return nil, err
	}
	return &w, nil
}

// ListByProjectID returns work streams for a project. statusFilter: "" or "active" = active only, "closed" = closed only, "all" = no filter.
func (s *Store) ListByProjectID(ctx context.Context, projectID string, statusFilter string) ([]WorkStream, error) {
	q := `SELECT id, project_id, name, slug, COALESCE(plan,''), COALESCE(branch,''), status, created_at
		  FROM work_streams WHERE project_id = $1`
	if statusFilter == "" || statusFilter == "active" {
		q += ` AND status = 'active'`
	} else if statusFilter == "closed" {
		q += ` AND status = 'closed'`
	}
	q += ` ORDER BY name`
	rows, err := s.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []WorkStream
	for rows.Next() {
		var w WorkStream
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Name, &w.Slug, &w.Plan, &w.Branch, &w.Status, &w.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// Update updates name, plan, branch, and status. Returns ErrWorkStreamNotFound if not found.
func (s *Store) Update(ctx context.Context, id string, name, plan, branch, status string) error {
	res, err := s.pool.Exec(ctx,
		`UPDATE work_streams SET name = $1, plan = $2, branch = $3, status = $4 WHERE id = $5`,
		name, nullIfEmpty(plan), nullIfEmpty(branch), status, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrWorkStreamNotFound
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
