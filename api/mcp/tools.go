package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/matt0x6f/warrant/api/rest"
	"github.com/matt0x6f/warrant/internal/auth"
	apierrors "github.com/matt0x6f/warrant/internal/errors"
	"github.com/matt0x6f/warrant/internal/execution"
	"github.com/matt0x6f/warrant/internal/gitnotes"
	"github.com/matt0x6f/warrant/internal/org"
	"github.com/matt0x6f/warrant/internal/project"
	"github.com/matt0x6f/warrant/internal/queue"
	"github.com/matt0x6f/warrant/internal/review"
	"github.com/matt0x6f/warrant/internal/ticket"
	"github.com/matt0x6f/warrant/internal/workstream"
)

type sessionContextKey struct{}

// RegisterTools adds all Warrant MCP tools to the MCP server (official go-sdk).
func RegisterTools(s *mcp.Server, b *Backend) {
	if b == nil {
		return
	}
	wrap := func(f func(*Backend, context.Context, map[string]any) (*mcp.CallToolResult, any, error)) func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
		return func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			if req != nil && req.Session != nil {
				ctx = context.WithValue(ctx, sessionContextKey{}, req.Session)
			}
			return f(b, ctx, args)
		}
	}

	mcp.AddTool(s, &mcp.Tool{Name: "list_orgs", Description: "List organizations you belong to. Returns id, name, slug for each. Requires OAuth. Use this to see your workspaces (e.g. personal org, or teams you were added to)."}, wrap(listOrgsHandler))

	mcp.AddTool(s, &mcp.Tool{Name: "create_project", Description: "Create a project in your default (first) organization. Use for initiatives, epics, or any work container. You do not pass org_id; the project is created in an org you belong to."}, wrap(createProjectHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "create_work_stream", Description: "Create a work stream in a project. Work streams group tickets toward a goal (e.g. 'Productionize feature A'). Warrant does not create a Git branch automatically: when the project has repo_url, the response includes git_instruction; create/checkout the branch locally, then call update_work_stream with branch so claim_ticket and get_ticket keep reminding you. Params: project_id, name (required), slug (optional), description (optional). Returns work_stream and optional git_instruction."}, wrap(createWorkStreamHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "list_work_streams", Description: "List work streams for a project. Params: project_id, optional status (active, closed, all)."}, wrap(listWorkStreamsHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "get_work_stream", Description: "Get a work stream by ID. Params: project_id, work_stream_id. When project has repo_url, response may include git_instruction (checkout branch, or create branch + update_work_stream if branch is not set yet)."}, wrap(getWorkStreamHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "update_work_stream", Description: "Update a work stream (name, description, branch, status). Set branch to the Git branch name after you create or choose it (required for persistent checkout hints on claim_ticket/get_ticket when repo_url is set). When closing (status=closed) and project has repo_url, returns git_instruction to checkout the project's default branch. Params: project_id, work_stream_id, optional name, description, branch, status (active|closed)."}, wrap(updateWorkStreamHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "create_ticket", Description: "Create a ticket in a project. The ticket is created as pending; agents can claim it via claim_ticket. created_by is set to your agent identity. Optional work_stream_id (associate with a work stream; if project has repo_url, ensure that work stream's branch is set via update_work_stream after creating the Git branch). Optional idempotency_key."}, wrap(createTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "list_projects", Description: "List projects for the authenticated user's organization(s). Requires OAuth (agent linked to a user). Returns only active projects by default. Pass include_closed: true to include closed projects. Optionally pass org_id to limit to one org (must be an org you belong to)."}, wrap(listProjectsHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "get_project_context", Description: "Return the full context pack for a project: conventions, key files, system prompt, and extra hints. Call this after list_projects to load the project's context before claiming or inspecting tickets."}, wrap(getProjectContextHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "update_project_status", Description: "Set a project's status to active or closed. Use to close a project when work is done, or reopen it (set to active) for follow-up. Requires OAuth and org access. Pass project_id and status (\"active\" or \"closed\"). Returns the updated project."}, wrap(updateProjectStatusHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "list_tickets", Description: "List tickets for a project. Optionally filter by state, priority (0–3), or work_stream_id. Use after get_project_context to see what work is available."}, wrap(listTicketsHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "get_ticket", Description: "Get the full ticket payload: objective, success criteria, acceptance test, context pack, dependency outputs (from tickets this one depends on), prior attempts, and human answers. This is the main input for doing the work. Call after claim_ticket and before start_ticket to load everything you need. If the ticket has a work_stream and the project has repo_url, the response may include git_instruction (checkout branch, or create branch + update_work_stream if branch is not set yet)."}, wrap(getTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "update_ticket", Description: "Update ticket metadata without calling REST. Minimum: update depends_on (project_id, ticket_id, depends_on list). Caller must have access to the project. Returns the updated ticket."}, wrap(updateTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "claim_ticket", Description: "Claim the next available ticket in the queue for a project. Returns the ticket and a lease (lease_token, expires_at). If the ticket has a work_stream and the project has repo_url, the response may include git_instruction (checkout branch, or create branch + update_work_stream if branch is not set yet). Optional idempotency_key: retries with the same key return the same ticket/lease (renewed if still valid) so the same agent does not claim a different ticket. You must start_ticket and then either submit_ticket or escalate_ticket before the lease expires, or renew_lease to extend. agent_id is inferred from OAuth when using URL auth."}, wrap(claimTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "start_ticket", Description: "Move the ticket from claimed to executing. Call after claim_ticket and get_ticket when you are ready to do the work. Requires the lease_token from claim_ticket. agent_id is inferred from OAuth when using URL auth."}, wrap(startTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "log_step", Description: "Append a step to the execution trace. Call this as you work—after each significant tool use (step_type tool_call, payload as object e.g. {\"name\":\"write\",\"input\":{\"path\":\"...\"}}), for key observations or decisions (observation/thought), and on errors (error). payload can be a JSON object or JSON string. Reviewers see this trace when approving the ticket; call it regularly so they know what was done."}, wrap(logStepHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "submit_ticket", Description: "Submit your outputs and move the ticket to awaiting_review. outputs must be a JSON object (e.g. {\"summary\":\"...\", \"artifacts\":[...]}). A human will approve or reject via the REST API. Call when the work is done."}, wrap(submitTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "escalate_ticket", Description: "Escalate to a human when you need help. Moves the ticket to needs_human. Provide a reason and a specific question; the human's answer is stored and the ticket returns to executing so you can continue. Use when blocked or when the objective is ambiguous."}, wrap(escalateTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "renew_lease", Description: "Extend the lease TTL so the ticket is not returned to the queue. Call periodically while working if the job takes longer than the lease duration. Returns the new expires_at."}, wrap(renewLeaseHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "force_release_lease", Description: "Force-release a ticket's lease (no token needed). Use when the user directs you to release a stuck ticket so you can claim it in this session (e.g. 'release agent-reliability-3 and claim it'). Caller must have access to the ticket's project. Ticket returns to pending; then use claim_ticket to claim it."}, wrap(forceReleaseLeaseHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "list_pending_reviews", Description: "List tickets in awaiting_review for a project. Use this when the user asks 'what needs my review?' or 'show pending reviews'. Returns full tickets so you can summarize them in chat; use get_trace(ticket_id) to show execution steps for each."}, wrap(listPendingReviewsHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "get_trace", Description: "Get the execution trace for a ticket (all log_step entries). Use when summarizing a ticket for review so the user can see what was done before approving or rejecting."}, wrap(getTraceHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "approve_ticket", Description: "Approve a ticket in awaiting_review. Moves it to done. Call when the user says to approve, ship it, looks good, etc. reviewer_id is inferred from OAuth."}, wrap(approveTicketHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "reject_ticket", Description: "Reject a ticket in awaiting_review. Returns it to executing with your notes appended so the agent can fix and resubmit. Call when the user says reject, needs changes, etc. reviewer_id is inferred from OAuth."}, wrap(rejectTicketHandler))

	// Git notes (Warrant integration): if repo_path provided and server has access, run git notes; else return commands for warrant-git CLI.
	mcp.AddTool(s, &mcp.Tool{Name: "warrant_add_git_note", Description: "Add a git note to a commit (refs/notes/warrant/decision|trace|intent). Params: message (required), type (decision|trace|intent, default decision), commit_sha (default HEAD), optional repo_path, ticket_id, project_id. If server has repo_path, adds note; else returns commands to run warrant-git note add locally."}, wrap(warrantAddGitNoteHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "warrant_show_git_notes", Description: "Show git note(s) for a commit. Params: commit_sha (default HEAD), optional repo_path, type (decision|trace|intent, or omit for all). Returns note body or commands for warrant-git note show."}, wrap(warrantShowGitNotesHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "warrant_log_git_notes", Description: "Log commits with notes (last N). Params: limit (default 20), optional repo_path, type (default decision). Returns list of {commit_sha, ref, body} or commands for warrant-git note log."}, wrap(warrantLogGitNotesHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "warrant_diff_git_notes", Description: "Notes on commits in base..head. Params: base, head (required), optional repo_path, type (default decision). Returns entries or commands for warrant-git note diff."}, wrap(warrantDiffGitNotesHandler))
	mcp.AddTool(s, &mcp.Tool{Name: "warrant_sync_git_notes", Description: "Push/pull refs/notes/warrant/*. Params: optional repo_path, direction (push|pull|both). Usually returns commands to run warrant-git sync locally."}, wrap(warrantSyncGitNotesHandler))
}

func requireString(args map[string]any, key string) (string, error) {
	if args == nil {
		return "", fmt.Errorf("%s required", key)
	}
	v, ok := args[key]
	if !ok || v == nil {
		return "", fmt.Errorf("%s required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}

func getString(args map[string]any, key, def string) string {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

func getInt(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return def
	}
}

func getBool(args map[string]any, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// getPayloadMap returns the payload for log_step as map[string]any. Accepts either a JSON string or an object so MCP clients can send payload as object and it is stored.
func getPayloadMap(args map[string]any, key string) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	v, ok := args[key]
	if !ok || v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if s, ok := v.(string); ok && s != "" {
		var out map[string]any
		if json.Unmarshal([]byte(s), &out) == nil {
			return out
		}
	}
	return map[string]any{}
}

func getAgentIDFromArgs(ctx context.Context, args map[string]any) (string, error) {
	if id := getString(args, "agent_id", ""); id != "" {
		return id, nil
	}
	if id := rest.GetAgentID(ctx); id != "" {
		return id, nil
	}
	// Stdio fallback: resolve agent ID from WARRANT_TOKEN env var (JWT).
	if token := os.Getenv("WARRANT_TOKEN"); token != "" {
		if secret := os.Getenv("JWT_SECRET"); secret != "" {
			if id, err := auth.VerifyJWT(secret, token); err == nil && id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("agent_id required (inferred from OAuth when using URL auth, or pass in request)")
}

// stuckTicketIDs returns ticket IDs in state claimed or executing for the project (so the user can direct force-release).
func stuckTicketIDs(ctx context.Context, b *Backend, projectID string) []string {
	claimed, _ := b.Ticket.ListByState(ctx, projectID, ticket.StateClaimed)
	executing, _ := b.Ticket.ListByState(ctx, projectID, ticket.StateExecuting)
	seen := make(map[string]bool)
	var ids []string
	for _, t := range claimed {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	for _, t := range executing {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// sessionFromContext returns the MCP ServerSession from ctx if present (for elicitation).
func sessionFromContext(ctx context.Context) *mcp.ServerSession {
	v := ctx.Value(sessionContextKey{})
	if v == nil {
		return nil
	}
	ss, _ := v.(*mcp.ServerSession)
	return ss
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInternal, err.Error(), false))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
}

func toolErrTriple(se *apierrors.StructuredError) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: se.JSON()}}}, nil, nil
}

func listOrgsHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	agent, err := b.AgentStore.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
	}
	if agent.UserID == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "list_orgs requires OAuth login (agent must be linked to a user). Use GitHub sign-in via MCP URL auth.", false))
	}
	orgs, err := b.Org.ListOrgsForUser(ctx, agent.UserID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	if orgs == nil {
		orgs = []*org.Org{}
	}
	return jsonResult(orgs)
}

func createProjectHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	agent, err := b.AgentStore.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
	}
	if agent.UserID == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "create_project requires OAuth login.", false))
	}
	orgs, err := b.Org.ListOrgsForUser(ctx, agent.UserID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	if len(orgs) == 0 {
		return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you have no organization; sign in again to ensure your default org exists", false))
	}
	name, err := requireString(args, "name")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	slug := getString(args, "slug", "")
		// Prefer personal org (slug "u-..."); otherwise use first org
		var targetOrg *org.Org
		for _, o := range orgs {
			if strings.HasPrefix(o.Slug, "u-") {
				targetOrg = o
				break
			}
		}
		if targetOrg == nil {
			targetOrg = orgs[0]
		}
		p, err := b.Project.CreateProject(ctx, targetOrg.ID, name, slug, "", nil)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
	}
	return jsonResult(p)
}

// workStreamGitInstruction returns checkout/create guidance when the project is git-backed.
// If ws.Branch is set, agents should check out that branch; otherwise suggest feature/<slug> and update_work_stream.
func workStreamGitInstruction(proj *project.Project, ws *workstream.WorkStream) map[string]any {
	if proj == nil || ws == nil || proj.RepoURL == "" {
		return nil
	}
	if ws.Branch != "" {
		return map[string]any{
			"enabled":          true,
			"action":           "checkout_branch",
			"suggested_branch": ws.Branch,
			"message":          "Check out branch '" + ws.Branch + "' before working. Create it if it doesn't exist: git checkout -b " + ws.Branch,
		}
	}
	if ws.Slug == "" {
		return nil
	}
	suggestedBranch := "feature/" + ws.Slug
	return map[string]any{
		"enabled":          true,
		"action":           "create_or_set_branch",
		"suggested_branch": suggestedBranch,
		"message":          "Create branch '" + suggestedBranch + "' with `git checkout -b " + suggestedBranch + "`, or if you are already on a branch, use `git branch --show-current` to get its name. Then call **update_work_stream** (project_id, work_stream_id, branch) with the branch name so future **claim_ticket** and **get_ticket** responses include checkout instructions.",
	}
}

func workStreamJSONWithGit(proj *project.Project, ws *workstream.WorkStream) (map[string]any, error) {
	raw, err := json.Marshal(ws)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	if gi := workStreamGitInstruction(proj, ws); gi != nil {
		out["git_instruction"] = gi
	}
	return out, nil
}

func attachWorkStreamGit(out map[string]any, proj *project.Project, ws *workstream.WorkStream) {
	if gi := workStreamGitInstruction(proj, ws); gi != nil {
		out["git_instruction"] = gi
	}
}

func createWorkStreamHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	if b.WorkStream == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInternal, "work stream service not configured", false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	name, err := requireString(args, "name")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	if err := checkProjectAccess(ctx, b, agentID, projectID); err != nil {
		return toolErrTriple(err)
	}
	slug := getString(args, "slug", "")
	description := getString(args, "description", "")
	w, err := b.WorkStream.CreateWorkStream(ctx, projectID, name, slug, description)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	out := map[string]any{"work_stream": w}
	if proj, _ := b.Project.GetProject(ctx, projectID); proj != nil {
		attachWorkStreamGit(out, proj, w)
	}
	return jsonResult(out)
}

func listWorkStreamsHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	if b.WorkStream == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInternal, "work stream service not configured", false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	if err := checkProjectAccess(ctx, b, agentID, projectID); err != nil {
		return toolErrTriple(err)
	}
	statusFilter := getString(args, "status", "active")
	list, err := b.WorkStream.ListWorkStreams(ctx, projectID, statusFilter)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	return jsonResult(list)
}

func getWorkStreamHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	if b.WorkStream == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInternal, "work stream service not configured", false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	workStreamID, err := requireString(args, "work_stream_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	if err := checkProjectAccess(ctx, b, agentID, projectID); err != nil {
		return toolErrTriple(err)
	}
	w, err := b.WorkStream.GetWorkStream(ctx, workStreamID)
	if err != nil {
		if errors.Is(err, workstream.ErrWorkStreamNotFound) {
			return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found", false))
		}
		return toolErrTriple(apierrors.MapError(err))
	}
	if w.ProjectID != projectID {
		return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found", false))
	}
	proj, _ := b.Project.GetProject(ctx, projectID)
	if m, err := workStreamJSONWithGit(proj, w); err == nil {
		return jsonResult(m)
	}
	return jsonResult(w)
}

func updateWorkStreamHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	if b.WorkStream == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInternal, "work stream service not configured", false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	workStreamID, err := requireString(args, "work_stream_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	if err := checkProjectAccess(ctx, b, agentID, projectID); err != nil {
		return toolErrTriple(err)
	}
	w, err := b.WorkStream.GetWorkStream(ctx, workStreamID)
	if err != nil {
		if errors.Is(err, workstream.ErrWorkStreamNotFound) {
			return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found", false))
		}
		return toolErrTriple(apierrors.MapError(err))
	}
	if w.ProjectID != projectID {
		return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found", false))
	}
	name := getString(args, "name", w.Name)
	description := getString(args, "description", w.Description)
	branch := getString(args, "branch", w.Branch)
	status := getString(args, "status", w.Status)
	if status == "" {
		status = w.Status
	}
	if err := b.WorkStream.UpdateWorkStream(ctx, workStreamID, name, description, branch, status); err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	updated, _ := b.WorkStream.GetWorkStream(ctx, workStreamID)
	out := map[string]any{"work_stream": updated}
	proj, _ := b.Project.GetProject(ctx, projectID)
	if status != "closed" && proj != nil && updated != nil {
		attachWorkStreamGit(out, proj, updated)
	}
	if status == "closed" {
		if proj != nil && proj.RepoURL != "" {
			defaultBranch := proj.DefaultBranch
			if defaultBranch == "" {
				defaultBranch = "main"
			}
			out["git_instruction"] = map[string]any{
				"enabled":  true,
				"action":   "checkout_default_branch",
				"branch":   defaultBranch,
				"message":  "Work stream closed. Checkout default branch with `git checkout " + defaultBranch + "`",
			}
		}
	}
	return jsonResult(out)
}

func checkProjectAccess(ctx context.Context, b *Backend, agentID, projectID string) *apierrors.StructuredError {
	agent, err := b.AgentStore.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return apierrors.New(apierrors.CodeUnauthorized, "agent not found", false)
	}
	if agent.UserID == "" {
		return apierrors.New(apierrors.CodeUnauthorized, "OAuth login required", false)
	}
	proj, err := b.Project.GetProject(ctx, projectID)
	if err != nil {
		return apierrors.MapError(err)
	}
	orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
	if err != nil {
		return apierrors.MapError(err)
	}
	for _, id := range orgIDs {
		if id == proj.OrgID {
			return nil
		}
	}
	return apierrors.New(apierrors.CodeForbidden, "you do not have access to that project", false)
}

func createTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		agentID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		projectID, err := requireString(args,"project_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		title, err := requireString(args,"title")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		description, err := requireString(args,"description")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		// Verify user has access to the project (project's org is one of user's orgs)
		proj, err := b.Project.GetProject(ctx, projectID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		agent, err := b.AgentStore.GetByID(ctx, agentID)
		if err != nil || agent == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
		}
		if agent.UserID == "" {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "create_ticket requires OAuth login.", false))
		}
		orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		allowed := false
		for _, id := range orgIDs {
			if id == proj.OrgID {
				allowed = true
				break
			}
		}
		if !allowed {
			return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you do not have access to that project", false))
		}
		if proj.Status == "closed" {
			return toolErrTriple(apierrors.New(apierrors.CodeProjectClosed, "project is closed; reopen it with update_project_status or choose another project", false))
		}
		typ := ticket.TypeTask
		if t := getString(args,"ticket_type", ""); t != "" {
			switch t {
			case "task", "bug", "spike", "review":
				typ = ticket.TicketType(t)
			}
		}
		prio := ticket.P2
		if p := getInt(args,"priority", -1); p >= 0 && p <= 3 {
			prio = ticket.Priority(p)
		}
		var successCriteria []string
		if s := getString(args,"success_criteria", ""); s != "" {
			_ = json.Unmarshal([]byte(s), &successCriteria)
		}
		acceptanceTest := getString(args,"acceptance_test", "")
		objective := ticket.Objective{
			Description:     description,
			SuccessCriteria: successCriteria,
			AcceptanceTest:  acceptanceTest,
		}
		idempotencyKey := getString(args, "idempotency_key", "")
		workStreamID := getString(args, "work_stream_id", "")
		if workStreamID != "" && b.WorkStream != nil {
			ws, err := b.WorkStream.GetWorkStream(ctx, workStreamID)
			if err != nil || ws == nil || ws.ProjectID != projectID {
				return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found or does not belong to project", false))
			}
		}
		dependsOn := []string{}
		if d := getString(args, "depends_on", ""); d != "" {
			_ = json.Unmarshal([]byte(d), &dependsOn)
		}
		t, err := b.Ticket.CreateTicket(ctx, projectID, title, typ, prio, agentID, dependsOn, workStreamID, objective, ticket.TicketContext{}, idempotencyKey)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(t)
}

func listProjectsHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		agentID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		agent, err := b.AgentStore.GetByID(ctx, agentID)
		if err != nil || agent == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
		}
		if agent.UserID == "" {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "list_projects requires OAuth login (agent must be linked to a user). Use GitHub sign-in via MCP URL auth.", false))
		}
		orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		if len(orgIDs) == 0 {
			return jsonResult([]any{})
		}
		filterOrgID := getString(args,"org_id", "")
		if filterOrgID != "" {
			allowed := false
			for _, id := range orgIDs {
				if id == filterOrgID {
					allowed = true
					break
				}
			}
			if !allowed {
				return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you are not a member of that organization", false))
			}
			orgIDs = []string{filterOrgID}
		}
		includeClosed := getBool(args, "include_closed", false)
		statusFilter := "active"
		if includeClosed {
			statusFilter = "all"
		}
		var all []project.Project
		for _, oid := range orgIDs {
			list, err := b.Project.ListByOrgID(ctx, oid, statusFilter)
			if err != nil {
				return toolErrTriple(apierrors.MapError(err))
			}
			all = append(all, list...)
		}
		return jsonResult(all)
}

func getProjectContextHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		projectID, err := requireString(args,"project_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		pack, err := b.Project.AssembleContextPack(ctx, projectID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(pack)
}

func updateProjectStatusHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	status, err := requireString(args, "status")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	if status != "active" && status != "closed" {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "status must be active or closed", false))
	}
	proj, err := b.Project.GetProject(ctx, projectID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	agent, err := b.AgentStore.GetByID(ctx, agentID)
	if err != nil || agent == nil {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
	}
	if agent.UserID == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "update_project_status requires OAuth login", false))
	}
	orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	allowed := false
	for _, id := range orgIDs {
		if id == proj.OrgID {
			allowed = true
			break
		}
	}
	if !allowed {
		return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you do not have access to that project", false))
	}
	if err := b.Project.UpdateStatus(ctx, projectID, status); err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	updated, err := b.Project.GetProject(ctx, projectID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	return jsonResult(updated)
}

func listTicketsHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		projectID, err := requireString(args,"project_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		workStreamID := getString(args, "work_stream_id", "")
		state := ticket.State(getString(args, "state", ""))
		list, err := b.Ticket.ListTickets(ctx, projectID, workStreamID, state)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		if p := getInt(args,"priority", -1); p >= 0 && p <= 3 {
			filtered := make([]*ticket.Ticket, 0)
			for _, t := range list {
				if int(t.Priority) == p {
					filtered = append(filtered, t)
				}
			}
			list = filtered
		}
		return jsonResult(list)
}

func getTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		t, err := b.Ticket.GetTicket(ctx, ticketID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		pack, _ := b.Project.AssembleContextPack(ctx, t.ProjectID)
		deps, _ := b.Ticket.GetTicketsByIDs(ctx, t.DependsOn)
		depOutputs := ticket.GetDependencyOutputs(t, deps)
		sum, _ := b.Trace.SummarizeTrace(ctx, ticketID)
		out := map[string]any{
			"ticket":              t,
			"context_pack":        pack,
			"dependency_outputs":  depOutputs,
			"prior_attempts":      t.Context.PriorAttempts,
			"human_answers":       t.Context.HumanAnswers,
		}
		if sum != nil {
			out["latest_attempt_summary"] = sum
		}
		if t.WorkStreamID != "" && b.WorkStream != nil {
			if ws, err := b.WorkStream.GetWorkStream(ctx, t.WorkStreamID); err == nil && ws != nil {
				out["work_stream"] = ws
				if proj, _ := b.Project.GetProject(ctx, t.ProjectID); proj != nil {
					attachWorkStreamGit(out, proj, ws)
				}
			}
		}
		return jsonResult(out)
}

func updateTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		agentID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		projectID, err := requireString(args,"project_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		agent, err := b.AgentStore.GetByID(ctx, agentID)
		if err != nil || agent == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
		}
		if agent.UserID == "" {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "update_ticket requires OAuth login.", false))
		}
		proj, err := b.Project.GetProject(ctx, projectID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		allowed := false
		for _, id := range orgIDs {
			if id == proj.OrgID {
				allowed = true
				break
			}
		}
		if !allowed {
			return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you do not have access to that project", false))
		}
		t, err := b.Ticket.GetTicket(ctx, ticketID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		if t.ProjectID != projectID {
			return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "ticket does not belong to that project", false))
		}
		dependsOnStr := getString(args,"depends_on", "")
		if dependsOnStr != "" {
			var dependsOn []string
			if err := json.Unmarshal([]byte(dependsOnStr), &dependsOn); err != nil {
				return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "depends_on must be a JSON array of ticket ID strings", false))
			}
			if err := b.Ticket.UpdateDependsOn(ctx, ticketID, dependsOn); err != nil {
				return toolErrTriple(apierrors.MapError(err))
			}
		}
		workStreamID := getString(args, "work_stream_id", "")
		if workStreamID != "" && b.WorkStream != nil {
			ws, err := b.WorkStream.GetWorkStream(ctx, workStreamID)
			if err != nil || ws == nil || ws.ProjectID != projectID {
				return toolErrTriple(apierrors.New(apierrors.CodeNotFound, "work stream not found or does not belong to project", false))
			}
			if err := b.Ticket.UpdateWorkStreamID(ctx, ticketID, workStreamID); err != nil {
				return toolErrTriple(apierrors.MapError(err))
			}
		}
		updated, err := b.Ticket.GetTicket(ctx, ticketID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(updated)
}

func claimTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	projectID, err := requireString(args, "project_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	proj, err := b.Project.GetProject(ctx, projectID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	if proj.Status == "closed" {
		return toolErrTriple(apierrors.New(apierrors.CodeProjectClosed, "project is closed; reopen it with update_project_status or choose another project", false))
	}
	var priority *int
	if p := getInt(args, "priority", -1); p >= 0 && p <= 3 {
		priority = &p
	}
	idempotencyKey := getString(args, "idempotency_key", "")
	t, lease, err := b.Queue.ClaimTicket(ctx, agentID, projectID, priority, idempotencyKey)
	if err != nil {
		if errors.Is(err, queue.ErrNoTicketAvailable) {
			if ss := sessionFromContext(ctx); ss != nil {
				// List tickets that are claimed or executing so the user can direct us to release one.
				stuckIDs := stuckTicketIDs(ctx, b, projectID)
				msg := "No ticket available to claim. Confirm below to have the agent retry, or choose a stuck ticket to force-release so the agent can claim it in this session."
				if len(stuckIDs) > 0 {
					msg = "No ticket available to claim. The following tickets may be stuck (claimed or executing): " + strings.Join(stuckIDs, ", ") + ". To direct the agent to claim one, set ticket_id_to_release to that ticket ID and confirm—the server will force-release it and retry claim."
				}
				schemaProps := map[string]any{
					"confirmed": map[string]any{"type": "boolean", "description": "Confirm to retry claim (and optionally release the chosen ticket)"},
				}
				if len(stuckIDs) > 0 {
					schemaProps["ticket_id_to_release"] = map[string]any{"type": "string", "description": "Optional: ticket ID to force-release so the agent can claim it (e.g. " + stuckIDs[0] + "). Leave empty to just retry."}
				}
				var elicitRes *mcp.ElicitResult
				var elicitErr error
				var elicitPanic error
				func() {
					defer func() {
						if r := recover(); r != nil {
							elicitPanic = fmt.Errorf("elicitation panic: %v", r)
						}
					}()
					elicitRes, elicitErr = ss.Elicit(ctx, &mcp.ElicitParams{
						Mode:            "form",
						Message:         msg,
						RequestedSchema: map[string]any{"type": "object", "properties": schemaProps},
					})
				}()
				if elicitPanic != nil {
					return toolErrTriple(apierrors.New(apierrors.CodeInternal, elicitPanic.Error(), true))
				}
				if elicitErr == nil && elicitRes != nil && elicitRes.Action == "accept" {
					content := elicitRes.Content
					if content == nil {
						content = map[string]any{}
					}
					if toRelease, _ := content["ticket_id_to_release"].(string); toRelease != "" {
						allowed := false
						for _, id := range stuckIDs {
							if id == toRelease {
								allowed = true
								break
							}
						}
						if allowed {
							_ = b.Queue.ForceReleaseLease(ctx, toRelease)
						}
					}
					t, lease, retryErr := b.Queue.ClaimTicket(ctx, agentID, projectID, priority, idempotencyKey)
					if retryErr == nil {
						return claimTicketResult(b, ctx, t, lease)
					}
				}
			}
		}
		return toolErrTriple(apierrors.MapError(err))
	}
	return claimTicketResult(b, ctx, t, lease)
}

func claimTicketResult(b *Backend, ctx context.Context, t *ticket.Ticket, lease *queue.Lease) (*mcp.CallToolResult, any, error) {
	out := map[string]any{"ticket": t, "lease": lease}
	if t.WorkStreamID != "" && b.WorkStream != nil {
		if ws, err := b.WorkStream.GetWorkStream(ctx, t.WorkStreamID); err == nil && ws != nil {
			out["work_stream"] = ws
			if proj, _ := b.Project.GetProject(ctx, t.ProjectID); proj != nil {
				attachWorkStreamGit(out, proj, ws)
			}
		}
	}
	return jsonResult(out)
}

func startTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		leaseToken, err := requireString(args,"lease_token")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		agentID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		actor := ticket.Actor{ID: agentID, Type: ticket.ActorAgent}
		if err := b.Ticket.TransitionTicket(ctx, ticketID, ticket.TriggerStart, actor, nil); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		_ = leaseToken
		return jsonResult(map[string]any{"ok": true})
}

func logStepHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		leaseToken, err := requireString(args,"lease_token")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		stepType, err := requireString(args,"step_type")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		payload := getPayloadMap(args, "payload")
		step := execution.Step{Type: execution.StepType(stepType), Payload: payload}
		if err := b.Trace.LogStep(ctx, ticketID, leaseToken, step); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"ok": true})
}

func submitTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		leaseToken, err := requireString(args,"lease_token")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		outputsStr, err := requireString(args,"outputs")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		var outputs map[string]any
		if err := json.Unmarshal([]byte(outputsStr), &outputs); err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, fmt.Sprintf("invalid outputs JSON: %v", err), false))
		}
		if err := b.Ticket.SubmitTicket(ctx, ticketID, leaseToken, outputs); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"ok": true})
}

func escalateTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		leaseToken, err := requireString(args,"lease_token")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		reason, _ := requireString(args,"reason")
		question, _ := requireString(args,"question")
		if err := b.Ticket.EscalateTicket(ctx, ticketID, leaseToken, reason, question); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"ok": true})
}

func renewLeaseHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		leaseToken, err := requireString(args,"lease_token")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		expiresAt, err := b.Queue.RenewLease(ctx, ticketID, leaseToken)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"expires_at": expiresAt})
}

func forceReleaseLeaseHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	ticketID, err := requireString(args, "ticket_id")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	agentID, err := getAgentIDFromArgs(ctx, args)
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	t, err := b.Ticket.GetTicket(ctx, ticketID)
	if err != nil || t == nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	proj, err := b.Project.GetProject(ctx, t.ProjectID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	agent, err := b.AgentStore.GetByID(ctx, agentID)
	if err != nil || agent == nil || agent.UserID == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "force_release_lease requires OAuth login", false))
	}
	orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
	if err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	allowed := false
	for _, id := range orgIDs {
		if id == proj.OrgID {
			allowed = true
			break
		}
	}
	if !allowed {
		return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you do not have access to that ticket's project", false))
	}
	if err := b.Queue.ForceReleaseLease(ctx, ticketID); err != nil {
		return toolErrTriple(apierrors.MapError(err))
	}
	return jsonResult(map[string]any{"ok": true, "ticket_id": ticketID, "message": "Ticket returned to pending; use claim_ticket to claim it."})
}

func listPendingReviewsHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		if b.Review == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInternal, "reviews not configured", false))
		}
		projectID, err := requireString(args,"project_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		agentID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		agent, err := b.AgentStore.GetByID(ctx, agentID)
		if err != nil || agent == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
		}
		if agent.UserID == "" {
			return toolErrTriple(apierrors.New(apierrors.CodeUnauthorized, "list_pending_reviews requires OAuth login.", false))
		}
		proj, err := b.Project.GetProject(ctx, projectID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		orgIDs, err := b.Org.ListOrgIDsForUser(ctx, agent.UserID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		allowed := false
		for _, id := range orgIDs {
			if id == proj.OrgID {
				allowed = true
				break
			}
		}
		if !allowed {
			return toolErrTriple(apierrors.New(apierrors.CodeForbidden, "you do not have access to that project", false))
		}
		ids, err := b.Review.ListPendingReviews(ctx, projectID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		tickets := make([]*ticket.Ticket, 0, len(ids))
		for _, id := range ids {
			t, _ := b.Ticket.GetTicket(ctx, id)
			if t != nil {
				tickets = append(tickets, t)
			}
		}
		return jsonResult(map[string]any{"tickets": tickets})
}

func getTraceHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		trace, err := b.Trace.GetTrace(ctx, ticketID)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(trace)
}

func approveTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		if b.Review == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInternal, "reviews not configured", false))
		}
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		reviewerID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		notes := getString(args,"notes", "")
		if err := b.Review.ApproveTicket(ctx, ticketID, reviewerID, notes); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"ok": true, "decision": review.DecisionApproved})
}

func rejectTicketHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
		if b.Review == nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInternal, "reviews not configured", false))
		}
		ticketID, err := requireString(args,"ticket_id")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		reviewerID, err := getAgentIDFromArgs(ctx, args)
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
		}
		notes, err := requireString(args,"notes")
		if err != nil {
			return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "notes required when rejecting (so the agent knows what to fix)", false))
		}
		if err := b.Review.RejectTicket(ctx, ticketID, reviewerID, notes); err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		return jsonResult(map[string]any{"ok": true, "decision": review.DecisionRejected})
}

// repoPathAccessible returns true if repoPath is non-empty and the path exists and is a git repo.
func repoPathAccessible(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(abs, ".git"))
	return err == nil
}

func warrantAddGitNoteHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	message, err := requireString(args, "message")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	noteType := getString(args, "type", gitnotes.TypeDecision)
	ref := gitnotes.RefForType(noteType)
	if ref == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "type must be decision, trace, or intent", false))
	}
	commitSHA := getString(args, "commit_sha", "HEAD")
	repoPath := getString(args, "repo_path", "")
	ticketID := getString(args, "ticket_id", "")
	projectID := getString(args, "project_id", "")
	agentID, _ := getAgentIDFromArgs(ctx, args)

	payload := map[string]any{
		"v":         1,
		"type":      noteType,
		"message":   message,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	if agentID != "" {
		payload["agent_id"] = agentID
	}
	if ticketID != "" {
		payload["ticket_id"] = ticketID
	}
	if projectID != "" {
		payload["project_id"] = projectID
	}
	bodyBytes, _ := json.Marshal(payload)
	body := string(bodyBytes)

	if repoPathAccessible(repoPath) {
		if err := gitnotes.AddNote(repoPath, ref, commitSHA, body); err != nil {
			return jsonResult(map[string]any{"ok": false, "error": err.Error(), "commands": warrantGitNoteAddCommands(noteType, message, commitSHA)})
		}
		return jsonResult(map[string]any{"ok": true, "message": "Note added."})
	}
	return jsonResult(map[string]any{"ok": true, "commands": warrantGitNoteAddCommands(noteType, message, commitSHA), "hint": "Run these in your repo (or install warrant-git and run the first)."})
}

func warrantGitNoteAddCommands(noteType, message, commitSHA string) []string {
	esc := strings.ReplaceAll(message, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	return []string{
		fmt.Sprintf(`warrant-git note add -t %s -m %q -c %s`, noteType, esc, commitSHA),
	}
}

func warrantShowGitNotesHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	commitSHA := getString(args, "commit_sha", "HEAD")
	repoPath := getString(args, "repo_path", "")
	noteType := getString(args, "type", "")

	if noteType != "" && gitnotes.RefForType(noteType) == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "type must be decision, trace, or intent", false))
	}

	if repoPathAccessible(repoPath) {
		if noteType != "" {
			ref := gitnotes.RefForType(noteType)
			if ref == "" {
				return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "type must be decision, trace, or intent", false))
			}
			body, err := gitnotes.ShowNote(repoPath, ref, commitSHA)
			if err != nil {
				return toolErrTriple(apierrors.MapError(err))
			}
			return jsonResult(map[string]any{"commit_sha": commitSHA, "ref": ref, "body": body})
		}
		out := make(map[string]any)
		out["commit_sha"] = commitSHA
		notes := make(map[string]string)
		for _, ref := range gitnotes.AllRefs() {
			body, _ := gitnotes.ShowNote(repoPath, ref, commitSHA)
			if body != "" {
				notes[filepath.Base(ref)] = body
			}
		}
		out["notes"] = notes
		return jsonResult(out)
	}
	cmd := fmt.Sprintf("warrant-git note show -c %s", commitSHA)
	if noteType != "" {
		cmd += " -t " + noteType
	}
	return jsonResult(map[string]any{"commands": []string{cmd}})
}

func warrantLogGitNotesHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	limit := getInt(args, "limit", 20)
	repoPath := getString(args, "repo_path", "")
	noteType := getString(args, "type", gitnotes.TypeDecision)
	ref := gitnotes.RefForType(noteType)
	if ref == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "type must be decision, trace, or intent", false))
	}

	if repoPathAccessible(repoPath) {
		entries, err := gitnotes.Log(repoPath, ref, limit)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		list := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			list = append(list, map[string]any{"commit_sha": e.CommitSHA, "ref": e.Ref, "body": e.Body})
		}
		return jsonResult(map[string]any{"entries": list})
	}
	return jsonResult(map[string]any{"commands": []string{fmt.Sprintf("warrant-git note log -t %s -n %d", noteType, limit)}})
}

func warrantDiffGitNotesHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	base, err := requireString(args, "base")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	head, err := requireString(args, "head")
	if err != nil {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, err.Error(), false))
	}
	repoPath := getString(args, "repo_path", "")
	noteType := getString(args, "type", gitnotes.TypeDecision)
	ref := gitnotes.RefForType(noteType)
	if ref == "" {
		return toolErrTriple(apierrors.New(apierrors.CodeInvalidInput, "type must be decision, trace, or intent", false))
	}

	if repoPathAccessible(repoPath) {
		entries, err := gitnotes.Diff(repoPath, ref, base, head)
		if err != nil {
			return toolErrTriple(apierrors.MapError(err))
		}
		list := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			list = append(list, map[string]any{"commit_sha": e.CommitSHA, "ref": e.Ref, "body": e.Body})
		}
		return jsonResult(map[string]any{"entries": list})
	}
	return jsonResult(map[string]any{"commands": []string{fmt.Sprintf("warrant-git note diff -t %s %s %s", noteType, base, head)}})
}

func warrantSyncGitNotesHandler(b *Backend, ctx context.Context, args map[string]any) (*mcp.CallToolResult, any, error) {
	direction := getString(args, "direction", "both")
	return jsonResult(map[string]any{
		"commands": []string{fmt.Sprintf("warrant-git sync %s", direction)},
		"hint":     "Run in your repo to push/pull refs/notes/warrant/*.",
	})
}
