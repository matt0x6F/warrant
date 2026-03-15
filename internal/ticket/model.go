package ticket

import "time"

// State is the ticket lifecycle state.
type State string

const (
	StatePending        State = "pending"
	StateClaimed        State = "claimed"
	StateExecuting      State = "executing"
	StateAwaitingReview State = "awaiting_review"
	StateDone           State = "done"
	StateBlocked        State = "blocked"
	StateNeedsHuman     State = "needs_human"
	StateFailed         State = "failed"
)

// TicketType is the kind of work.
type TicketType string

const (
	TypeTask   TicketType = "task"
	TypeBug    TicketType = "bug"
	TypeSpike  TicketType = "spike"
	TypeReview TicketType = "review"
)

// Priority 0 = P0 (highest), 3 = P3 (lowest).
type Priority int

const (
	P0 Priority = 0
	P1 Priority = 1
	P2 Priority = 2
	P3 Priority = 3
)

// Objective describes what the ticket is for.
type Objective struct {
	Description     string   `json:"description"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
	AcceptanceTest  string   `json:"acceptance_test,omitempty"`
}

// AcceptanceTest is the command or check (stored in Objective for v1).
// Kept as type alias for plan compatibility.
type AcceptanceTest = string

// TicketContext holds relevant files, constraints, prior attempts, human answers.
type TicketContext struct {
	RelevantFiles  []string        `json:"relevant_files,omitempty"`
	Constraints    []string        `json:"constraints,omitempty"`
	PriorAttempts  []AttemptSummary `json:"prior_attempts,omitempty"`
	HumanAnswers   []string        `json:"human_answers,omitempty"` // from resolved escalations
}

// AttemptSummary is a short summary of a prior execution attempt (for context injection).
type AttemptSummary struct {
	AgentID   string `json:"agent_id"`
	Outcome   string `json:"outcome"` // e.g. "rejected", "failed"
	Summary   string `json:"summary,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// Lease represents an agent's claim on a ticket (TTL-based).
type Lease struct {
	TicketID  string
	AgentID   string
	Token     string
	ExpiresAt time.Time
	Renewable bool
}

// Ticket is the core entity.
type Ticket struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"project_id"`
	Title        string       `json:"title"`
	Type         TicketType    `json:"type"`
	Priority     Priority     `json:"priority"`
	State        State        `json:"state"`
	Version      int          `json:"version"`
	Objective    Objective    `json:"objective"`
	Context       TicketContext   `json:"ticket_context"`
	Inputs       map[string]any `json:"inputs"`
	Outputs      map[string]any `json:"outputs"`
	DependsOn    []string     `json:"depends_on"`
	WorkStreamID string      `json:"work_stream_id,omitempty"`
	AssignedTo   string      `json:"assigned_to,omitempty"`
	CreatedBy    string      `json:"created_by"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}
