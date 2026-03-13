package mcp

import (
	"github.com/matt0x6f/warrant/internal/agent"
	"github.com/matt0x6f/warrant/internal/execution"
	"github.com/matt0x6f/warrant/internal/org"
	"github.com/matt0x6f/warrant/internal/project"
	"github.com/matt0x6f/warrant/internal/queue"
	"github.com/matt0x6f/warrant/internal/review"
	"github.com/matt0x6f/warrant/internal/ticket"
)

// Backend holds the services needed by MCP tools. Set by the server that runs MCP.
type Backend struct {
	Project    *project.Service
	Ticket     *ticket.Service
	Queue      *queue.Service
	Trace      *execution.Service
	Review     *review.Service
	Org        *org.Service
	AgentStore *agent.Store
}
