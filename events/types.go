package events

// Event type constants and payloads.
const (
	EventTicketCreated   = "ticket.created"
	EventTicketClaimed   = "ticket.claimed"
	EventTicketStarted   = "ticket.started"
	EventTicketSubmitted = "ticket.submitted"
	EventTicketApproved  = "ticket.approved"
	EventTicketRejected  = "ticket.rejected"
	EventTicketFailed    = "ticket.failed"
	EventTicketEscalated = "ticket.escalated"
	EventTicketDone      = "ticket.done"
	EventLeaseExpired    = "lease.expired"
	EventTicketUnblocked = "ticket.unblocked"
)

// Event carries type and opaque payload for the bus.
type Event struct {
	Type    string
	Payload map[string]any
}
