package queue

import "time"

// Lease represents an agent's claim on a ticket (TTL-based, stored in Redis).
type Lease struct {
	TicketID  string    `json:"ticket_id"`
	AgentID   string    `json:"agent_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Renewable bool      `json:"renewable"`
}
