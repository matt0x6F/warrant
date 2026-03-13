package review

import "time"

// Decision is the outcome of a review.
type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionRejected Decision = "rejected"
)

// Review records a human review of a ticket (approve/reject).
type Review struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticket_id"`
	ReviewerID string   `json:"reviewer_id"`
	Decision   Decision `json:"decision"`
	Notes      string   `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Escalation records an agent asking for human help.
type Escalation struct {
	ID         string     `json:"id"`
	TicketID   string     `json:"ticket_id"`
	AgentID    string     `json:"agent_id"`
	Reason     string     `json:"reason"`
	Question   string     `json:"question"`
	Answer     string     `json:"answer,omitempty"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
