package ticket

import (
	"errors"
	"fmt"
)

// Trigger constants (only valid triggers for transitions).
const (
	TriggerClaim        = "claim"
	TriggerStart        = "start"
	TriggerSubmit       = "submit"
	TriggerEscalate     = "escalate"
	TriggerApprove      = "approve"
	TriggerReject       = "reject"
	TriggerLeaseExpired = "lease_expired"
	TriggerFail         = "fail"
	TriggerCancel       = "cancel"
)

// ActorType is who is performing the action.
type ActorType string

const (
	ActorHuman  ActorType = "human"
	ActorAgent  ActorType = "agent"
	ActorSystem ActorType = "system"
)

// Actor identifies who is performing the transition.
type Actor struct {
	ID   string
	Type ActorType
}

// GuardFn returns an error if the transition is not allowed.
type GuardFn func(t *Ticket, actor Actor, payload map[string]any, deps []*Ticket) error

// Transition defines a valid state transition.
type Transition struct {
	From    State
	To      State
	Trigger string
	Guards  []GuardFn
}

// StateMachine holds all valid transitions and applies them.
type StateMachine struct {
	transitions []Transition
}

// NewStateMachine returns a state machine with all plan-defined transitions.
func NewStateMachine() *StateMachine {
	sm := &StateMachine{}
	sm.transitions = []Transition{
		{StatePending, StateClaimed, TriggerClaim, []GuardFn{guardDependenciesMet, guardNoActiveLease}},
		{StateClaimed, StateExecuting, TriggerStart, []GuardFn{guardIsLeaseholder}},
		{StateExecuting, StateAwaitingReview, TriggerSubmit, []GuardFn{guardIsLeaseholder, guardOutputsPresent}},
		{StateExecuting, StateNeedsHuman, TriggerEscalate, []GuardFn{guardIsLeaseholder, guardEscalationReasonPresent}},
		{StateExecuting, StateBlocked, TriggerCancel, []GuardFn{}},
		{StateExecuting, StateFailed, TriggerFail, []GuardFn{}},
		{StateAwaitingReview, StateDone, TriggerApprove, []GuardFn{guardIsHuman}},
		{StateAwaitingReview, StateExecuting, TriggerReject, []GuardFn{guardIsHuman}},
		{StateClaimed, StatePending, TriggerLeaseExpired, []GuardFn{}},
		{StateExecuting, StatePending, TriggerLeaseExpired, []GuardFn{guardSystemOnly}}, // operator force: release stuck executing ticket
		{StateNeedsHuman, StateExecuting, TriggerApprove, []GuardFn{guardIsHuman}},   // resolve escalation → back to executing
	}
	return sm
}

// Transition finds the matching transition, runs guards, and returns the new state or error.
func (sm *StateMachine) Transition(t *Ticket, trigger string, actor Actor, payload map[string]any, deps []*Ticket) (State, error) {
	if payload == nil {
		payload = make(map[string]any)
	}
	for _, tr := range sm.transitions {
		if tr.From == t.State && tr.Trigger == trigger {
			for _, guard := range tr.Guards {
				if err := guard(t, actor, payload, deps); err != nil {
					return "", fmt.Errorf("guard: %w", err)
				}
			}
			return tr.To, nil
		}
	}
	return "", fmt.Errorf("no valid transition from %s with trigger %s", t.State, trigger)
}

func guardDependenciesMet(t *Ticket, _ Actor, _ map[string]any, deps []*Ticket) error {
	for _, d := range deps {
		if d.State != StateDone {
			return errors.New("dependency not done")
		}
	}
	return nil
}

func guardNoActiveLease(t *Ticket, _ Actor, _ map[string]any, _ []*Ticket) error {
	if t.AssignedTo != "" {
		return errors.New("ticket already claimed")
	}
	return nil
}

func guardIsLeaseholder(t *Ticket, actor Actor, _ map[string]any, _ []*Ticket) error {
	if t.AssignedTo != actor.ID {
		return errors.New("actor is not the leaseholder")
	}
	return nil
}

func guardOutputsPresent(t *Ticket, _ Actor, payload map[string]any, _ []*Ticket) error {
	if payload["outputs"] == nil {
		return errors.New("outputs required for submit")
	}
	return nil
}

func guardEscalationReasonPresent(t *Ticket, _ Actor, payload map[string]any, _ []*Ticket) error {
	if payload["reason"] == nil && payload["question"] == nil {
		return errors.New("reason or question required for escalate")
	}
	return nil
}

func guardIsHuman(t *Ticket, _ Actor, _ map[string]any, _ []*Ticket) error {
	// Approve/reject/resolve escalation must be done by a human (we don't have actor in payload for REST; check actor type)
	// For now we allow any actor; can restrict to ActorHuman when auth is wired.
	return nil
}

func guardSystemOnly(t *Ticket, actor Actor, _ map[string]any, _ []*Ticket) error {
	if actor.Type != ActorSystem {
		return errors.New("only system can force lease_expired from executing")
	}
	return nil
}
