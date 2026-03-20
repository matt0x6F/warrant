package mcp

// Workflow is a small JSON object included in some MCP tool results to nudge agents
// through claim → start → log_step → submit without replacing the agent guide.
type Workflow struct {
	NextSteps []string `json:"next_steps"`
	Note      string   `json:"note,omitempty"`
}

func workflowAfterCreateTicket(projectID string) Workflow {
	return Workflow{
		NextSteps: []string{
			"claim_ticket",
			"start_ticket",
			"log_step",
			"submit_ticket",
		},
		Note: "Ticket is pending. To implement: claim_ticket(project_id), then start_ticket(lease_token), log_step while working, submit_ticket when done. Do not skip claim/start.",
	}
}

func workflowAfterClaim() Workflow {
	return Workflow{
		NextSteps: []string{
			"get_ticket",
			"start_ticket",
			"log_step",
			"submit_ticket",
		},
		Note: "You hold the lease. Call start_ticket before coding; renew_lease if needed; log_step during work; submit_ticket when finished.",
	}
}

func workflowAfterStart() Workflow {
	return Workflow{
		NextSteps: []string{
			"log_step",
			"submit_ticket",
		},
		Note: "Ticket is executing. Log steps for reviewers; submit_ticket outputs JSON when done.",
	}
}

func workflowAfterListTicketsPending() Workflow {
	return Workflow{
		NextSteps: []string{
			"claim_ticket",
			"start_ticket",
			"log_step",
			"submit_ticket",
		},
		Note: "To work a ticket: claim_ticket(project_id)—queue-ordered; do not implement only in git without claiming.",
	}
}
