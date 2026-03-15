package mcp

import (
	"github.com/matt0x6f/warrant/internal/agent"
	"github.com/matt0x6f/warrant/internal/execution"
	"github.com/matt0x6f/warrant/internal/org"
	"github.com/matt0x6f/warrant/internal/project"
	"github.com/matt0x6f/warrant/internal/queue"
	"github.com/matt0x6f/warrant/internal/workstream"
	"github.com/matt0x6f/warrant/internal/review"
	"github.com/matt0x6f/warrant/internal/ticket"
)

// Backend holds the services needed by MCP tools. Set by the server that runs MCP.
type Backend struct {
	Project    *project.Service
	WorkStream *workstream.Service
	Ticket     *ticket.Service
	Queue      *queue.Service
	Trace      *execution.Service
	Review     *review.Service
	Org        *org.Service
	AgentStore *agent.Store

	// DefaultAgentID is used as a fallback when agent_id is not passed in args
	// and not available from HTTP auth context (e.g. stdio mode with WARRANT_TOKEN).
	DefaultAgentID string
}
