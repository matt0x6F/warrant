package mcp

import (
	"context"
	"testing"
)

func TestUpdateWorkStreamPlanHandler_NoService(t *testing.T) {
	b := &Backend{}
	ctx := context.Background()
	res, _, err := updateWorkStreamPlanHandler(b, ctx, map[string]any{
		"project_id":     "p1",
		"work_stream_id": "ws1",
		"plan":           "x",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected error when work stream service not configured")
	}
}

func TestUpdateWorkStreamPlanHandler_MissingPlan(t *testing.T) {
	b := &Backend{}
	ctx := context.Background()
	res, _, err := updateWorkStreamPlanHandler(b, ctx, map[string]any{
		"project_id":     "p1",
		"work_stream_id": "ws1",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected error when plan missing")
	}
}
