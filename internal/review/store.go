package review

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists reviews and escalations.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a new Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateReview inserts a review.
func (s *Store) CreateReview(ctx context.Context, r *Review) error {
	r.ID = uuid.Must(uuid.NewV7()).String()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO reviews (id, ticket_id, reviewer_id, decision, notes, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ID, r.TicketID, r.ReviewerID, r.Decision, r.Notes, r.CreatedAt)
	return err
}

// CreateEscalation inserts an escalation.
func (s *Store) CreateEscalation(ctx context.Context, e *Escalation) error {
	e.ID = uuid.Must(uuid.NewV7()).String()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO escalations (id, ticket_id, agent_id, reason, question, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		e.ID, e.TicketID, e.AgentID, e.Reason, e.Question, e.CreatedAt)
	return err
}

// UpdateEscalationResolved sets answer and resolved_by/at.
func (s *Store) UpdateEscalationResolved(ctx context.Context, id, answer, resolvedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE escalations SET answer = $1, resolved_by = $2, resolved_at = now() WHERE id = $3`,
		answer, resolvedBy, id)
	return err
}

// ListReviewsByTicket returns reviews for a ticket.
func (s *Store) ListReviewsByTicket(ctx context.Context, ticketID string) ([]Review, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, ticket_id, reviewer_id, decision, notes, created_at FROM reviews WHERE ticket_id = $1 ORDER BY created_at`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.TicketID, &r.ReviewerID, &r.Decision, &r.Notes, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// ListPendingReviewTicketIDs returns ticket IDs in state awaiting_review for a project.
func (s *Store) ListPendingReviewTicketIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM tickets WHERE project_id = $1 AND state = 'awaiting_review' ORDER BY updated_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListEscalationsByProject returns unresolved escalations for a project.
func (s *Store) ListEscalationsByProject(ctx context.Context, projectID string) ([]Escalation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT e.id, e.ticket_id, e.agent_id, e.reason, e.question, e.answer, e.resolved_by, e.resolved_at, e.created_at
		 FROM escalations e JOIN tickets t ON t.id = e.ticket_id WHERE t.project_id = $1 AND e.resolved_at IS NULL ORDER BY e.created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Escalation
	for rows.Next() {
		var e Escalation
		var answer, resolvedBy *string
		var resolvedAt *time.Time
		if err := rows.Scan(&e.ID, &e.TicketID, &e.AgentID, &e.Reason, &e.Question, &answer, &resolvedBy, &resolvedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		if answer != nil {
			e.Answer = *answer
		}
		if resolvedBy != nil {
			e.ResolvedBy = *resolvedBy
		}
		if resolvedAt != nil {
			e.ResolvedAt = resolvedAt
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// GetEscalationByID returns one escalation.
func (s *Store) GetEscalationByID(ctx context.Context, id string) (*Escalation, error) {
	var e Escalation
	var answer, resolvedBy *string
	var resolvedAt *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT id, ticket_id, agent_id, reason, question, answer, resolved_by, resolved_at, created_at FROM escalations WHERE id = $1`, id).
		Scan(&e.ID, &e.TicketID, &e.AgentID, &e.Reason, &e.Question, &answer, &resolvedBy, &resolvedAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if answer != nil {
		e.Answer = *answer
	}
	if resolvedBy != nil {
		e.ResolvedBy = *resolvedBy
	}
	if resolvedAt != nil {
		e.ResolvedAt = resolvedAt
	}
	return &e, nil
}
