package ticket

import (
	"testing"
)

func TestStateMachine_Transition(t *testing.T) {
	sm := NewStateMachine()

	tests := []struct {
		name        string
		from        State
		trigger     string
		actor       Actor
		assignedTo  string // ticket.AssignedTo (for leaseholder checks)
		payload     map[string]any
		deps        []*Ticket
		wantState   State
		wantErr     bool
	}{
		{"pending->claimed", StatePending, TriggerClaim, Actor{ID: "agent1", Type: ActorAgent}, "", nil, nil, StateClaimed, false},
		{"pending->claimed with deps done", StatePending, TriggerClaim, Actor{ID: "a", Type: ActorAgent}, "", map[string]any{"agent_id": "a"}, []*Ticket{{ID: "x", State: StateDone}}, StateClaimed, false},
		{"pending->claimed dep not done", StatePending, TriggerClaim, Actor{ID: "a", Type: ActorAgent}, "", nil, []*Ticket{{ID: "x", State: StateExecuting}}, "", true},
		{"claimed->executing", StateClaimed, TriggerStart, Actor{ID: "agent1", Type: ActorAgent}, "agent1", nil, nil, StateExecuting, false},
		{"claimed->executing wrong agent", StateClaimed, TriggerStart, Actor{ID: "other", Type: ActorAgent}, "agent1", nil, nil, "", true},
		{"executing->awaiting_review", StateExecuting, TriggerSubmit, Actor{ID: "agent1", Type: ActorAgent}, "agent1", map[string]any{"outputs": map[string]any{"x": 1}}, nil, StateAwaitingReview, false},
		{"executing->submit no outputs", StateExecuting, TriggerSubmit, Actor{ID: "agent1", Type: ActorAgent}, "agent1", nil, nil, "", true},
		{"executing->needs_human", StateExecuting, TriggerEscalate, Actor{ID: "agent1", Type: ActorAgent}, "agent1", map[string]any{"reason": "stuck", "question": "?"}, nil, StateNeedsHuman, false},
		{"executing->escalate no reason", StateExecuting, TriggerEscalate, Actor{ID: "agent1", Type: ActorAgent}, "agent1", nil, nil, "", true},
		{"awaiting_review->done", StateAwaitingReview, TriggerApprove, Actor{ID: "human1", Type: ActorHuman}, "agent1", nil, nil, StateDone, false},
		{"done->awaiting_review reopen", StateDone, TriggerReopenReview, Actor{ID: "human1", Type: ActorHuman}, "agent1", nil, nil, StateAwaitingReview, false},
		{"awaiting_review->executing reject", StateAwaitingReview, TriggerReject, Actor{ID: "human1", Type: ActorHuman}, "agent1", nil, nil, StateExecuting, false},
		{"claimed->pending lease_expired", StateClaimed, TriggerLeaseExpired, Actor{ID: "system", Type: ActorSystem}, "agent1", nil, nil, StatePending, false},
		{"executing->pending lease_expired (system)", StateExecuting, TriggerLeaseExpired, Actor{ID: "operator", Type: ActorSystem}, "agent1", nil, nil, StatePending, false},
		{"executing->pending lease_expired non-system rejected", StateExecuting, TriggerLeaseExpired, Actor{ID: "agent1", Type: ActorAgent}, "agent1", nil, nil, "", true},
		{"executing->failed", StateExecuting, TriggerFail, Actor{ID: "agent1", Type: ActorAgent}, "agent1", nil, nil, StateFailed, false},
		{"invalid trigger", StatePending, "invalid", Actor{}, "", nil, nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &Ticket{State: tt.from}
			if tt.assignedTo != "" {
				ticket.AssignedTo = tt.assignedTo
			} else if tt.actor.ID != "" && (tt.from == StateClaimed || tt.from == StateExecuting || tt.from == StateAwaitingReview) {
				ticket.AssignedTo = tt.actor.ID
			}
			got, err := sm.Transition(ticket, tt.trigger, tt.actor, tt.payload, tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantState {
				t.Errorf("Transition() got state %v, want %v", got, tt.wantState)
			}
		})
	}
}
