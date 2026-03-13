package execution

import (
	"context"
	"time"
)

// LeaseValidator returns the agent ID that holds the lease for the ticket, or an error.
type LeaseValidator interface {
	ValidateLease(ctx context.Context, ticketID, token string) (agentID string, err error)
}

// Service provides execution trace operations.
type Service struct {
	store   *Store
	validate LeaseValidator
}

// NewService returns a new Service.
func NewService(store *Store, validate LeaseValidator) *Service {
	return &Service{store: store, validate: validate}
}

// LogStep validates the lease and appends a step to the ticket's trace.
func (s *Service) LogStep(ctx context.Context, ticketID, leaseToken string, step Step) error {
	agentID, err := s.validate.ValidateLease(ctx, ticketID, leaseToken)
	if err != nil {
		return err
	}
	if step.CreatedAt.IsZero() {
		step.CreatedAt = time.Now().UTC()
	}
	return s.store.AppendStep(ctx, ticketID, agentID, step)
}

// GetTrace returns the full execution history for a ticket.
func (s *Service) GetTrace(ctx context.Context, ticketID string) (*ExecutionTrace, error) {
	steps, err := s.store.GetStepsByTicketID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	agentID := ""
	if len(steps) > 0 {
		agentID, _ = s.store.GetAgentIDByTicketID(ctx, ticketID)
	}
	return &ExecutionTrace{TicketID: ticketID, AgentID: agentID, Steps: steps}, nil
}

// SummarizeTrace produces an AttemptSummary for the latest run (for prior_attempts context injection).
func (s *Service) SummarizeTrace(ctx context.Context, ticketID string) (*AttemptSummary, error) {
	steps, err := s.store.GetStepsByTicketID(ctx, ticketID)
	if err != nil || len(steps) == 0 {
		return nil, err
	}
	agentID, _ := s.store.GetAgentIDByTicketID(ctx, ticketID)
	last := steps[len(steps)-1]
	outcome := string(last.Type)
	if outcome == "" {
		outcome = "unknown"
	}
	summary := ""
	if s, ok := last.Payload["summary"].(string); ok {
		summary = s
	}
	return &AttemptSummary{
		AgentID:   agentID,
		Outcome:   outcome,
		Summary:   summary,
		CreatedAt: last.CreatedAt.Format(time.RFC3339),
	}, nil
}
