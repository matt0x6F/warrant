package mcp

import (
	"encoding/json"
	"testing"
)

func TestWorkflow_JSON(t *testing.T) {
	w := workflowAfterCreateTicket("proj-1")
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var got Workflow
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.NextSteps) == 0 {
		t.Fatal("expected next_steps")
	}
	if got.Note == "" {
		t.Fatal("expected note")
	}
}
