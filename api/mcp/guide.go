package mcp

// AgentGuideURI is the URI for the MCP resource that describes the typical agent flow.
const AgentGuideURI = "warrant://docs/agent-guide"

// AgentGuideContent is the markdown content for the agent guide resource.
// Uses (param) instead of `param` so the string can be a single raw literal.
const AgentGuideContent = `# Warrant MCP – Agent guide

Use this flow when working on tickets via Warrant. Your identity is tied to your OAuth login; you only see projects in organizations you belong to.

## Typical flow

1. **list_projects** (no args when using OAuth) – Returns projects in all organizations you are a member of. Optionally pass (org_id) to limit to one org.
2. **get_project_context** (project_id) – Load conventions, key files, system prompt, and extra hints for the project.
3. **list_tickets** (project_id, optional state, priority) – See available tickets. Filter by state (e.g. pending) or priority (0–3).
4. **claim_ticket** (project_id) – Claim the next available ticket. Returns the ticket and a **lease** (lease_token, expires_at). agent_id is inferred from OAuth.
5. **get_ticket** (ticket_id) – Load the full payload: objective, success criteria, acceptance test, context pack, dependency outputs, prior attempts, human answers. This is your main input for doing the work.
6. **start_ticket** (ticket_id, lease_token) – Move the ticket to **executing**.
7. **While working, interleave log_step** so reviewers see what you did:
   - After each significant tool or action: **log_step** with step_type **tool_call** and payload (e.g. name, input).
   - For key findings or decisions: **log_step** with **observation** or **thought** and a short payload (e.g. summary).
   - On failure: **log_step** with step_type **error** and payload describing the error.
   Use **renew_lease** if the job takes longer than the lease TTL.
8. When done:
   - **submit_ticket** (ticket_id, lease_token, outputs) – Submit deliverables (JSON) and move to **awaiting_review**. A human approves or rejects via REST.
   - **escalate_ticket** (ticket_id, lease_token, reason, question) – Ask for human help; ticket moves to **needs_human**. The human's answer is stored and the ticket returns to **executing**.

## Tool summary

| Tool | Purpose |
|------|--------|
| list_orgs | List organizations you belong to (id, name, slug). OAuth required. |
| create_project | Create a project in your default org (name, optional slug). For initiatives/epics. |
| list_projects | List projects for your org(s). Default: active only; pass include_closed: true to include closed. OAuth required; org_id optional. |
| update_project_status | Set project status to active or closed (project_id, status). Use to close when done or reopen for follow-up. |
| create_ticket | Create a pending ticket in a project (project_id, title, description, optional type/priority). Fails if project is closed. |
| get_project_context | Context pack for a project (conventions, key files, system prompt). |
| list_tickets | List tickets by project; optional state/priority filter. |
| get_ticket | Full ticket + context + dependency outputs + prior attempts. |
| claim_ticket | Claim next ticket; returns ticket + lease. If none available, elicitation may ask you to force-release a stuck ticket. |
| force_release_lease | Force-release a ticket's lease (no token). Use when the user says "release X and claim it"; then call claim_ticket. |
| start_ticket | Move claimed to executing. |
| log_step | Append a step to the execution trace. Call as you work (after tool use, observations, errors) so reviewers see what was done. |
| submit_ticket | Submit outputs; move to awaiting_review. If the ticket has **objective.acceptance_test** and the server has it enabled, the server runs that command (e.g. shell script) before transitioning; on failure the submit is rejected with the test output so you can fix and retry. |
| escalate_ticket | Ask for human help; move to needs_human. |
| renew_lease | Extend lease TTL. |
| list_pending_reviews | List tickets in awaiting_review for a project. Use when the user asks "what needs my review?" or "show pending reviews". |
| get_trace | Get the execution trace for a ticket (all log_step entries). Use when summarizing a ticket for review. |
| approve_ticket | Approve a ticket in awaiting_review (moves to done). Use when the user says approve, ship it, looks good. |
| reject_ticket | Reject a ticket with required notes; returns to executing so the agent can fix and resubmit. |

## Ticket states

pending → claimed → executing → awaiting_review → done. Side states: blocked, needs_human, failed.

## Finishing a project

A project is **finished** when: (1) all tickets are in state **done** (no pending, claimed, executing, or awaiting_review), and (2) you have no more work to add. Set the project status to **closed** via **update_project_status** (project_id, status: "closed") so it no longer appears in the default **list_projects** and so new tickets cannot be created or claimed for it. To reopen for follow-up work, call **update_project_status** with status **active**. Optionally create a final ticket (e.g. "Project complete – [initiative name]") and approve it before closing. For initiatives tracked in docs (e.g. docs/initiatives/), update the doc or add a "Done" note when the project is finished.

## Review visibility

When you **submit_ticket**, a human reviews it. They see your **outputs** (the JSON you passed) and the **execution trace** (every **log_step** you sent). If you never call log_step, the trace is empty and the reviewer has no record of what you did—only the final outputs. Interleaving log_step as you work gives reviewers a clear picture of your actions and decisions.

## Writing traces (log_step)

Call **log_step** frequently so the execution trace is useful:

- **After significant actions**: use step_type **tool_call** and pass **payload** as an object, e.g. (name, input) for a write or run_terminal_cmd. The server accepts payload as either a JSON object or a JSON string.
- **For decisions or findings**: use step_type **observation** or **thought** with a short payload (e.g. summary text).
- **On errors**: use step_type **error** with payload describing what failed.

Without these steps, **get_trace** returns an empty list and reviewers cannot see what was done.

## Git notes after submit

When you complete work and call **submit_ticket**, if the user's repo is the project repo (or you have a repo_path), add a **git note** so the commit records what was done. Use **warrant_add_git_note** with type **decision**, message = one-line summary of the work, and optional ticket_id/project_id. If the server cannot access the repo, the tool returns commands to run **warrant-git note add** locally—surface those to the user or run them in the workspace. That way refs/notes/warrant/decision (and optionally trace/intent) stay in sync with completed work.

## Reviews in conversation

The user can review work entirely in chat. When they ask **"What needs my review?"** or **"Show me pending reviews"**:

1. Call **list_pending_reviews** (project_id) for the relevant project (or for each project they care about). You get back tickets in awaiting_review.
2. For each ticket, summarize: title, objective, and **outputs**. Optionally call **get_trace** (ticket_id) and summarize key steps so they see what was done.
3. When the user says to **approve** (e.g. "Approve it", "Ship it", "Looks good"): call **approve_ticket** (ticket_id, optional notes).
4. When they say to **reject** (e.g. "Reject", "Needs changes"): call **reject_ticket** (ticket_id, notes) — notes are required so the agent knows what to fix.
</think>

## Human-in-the-loop (elicitation)

When **claim_ticket** returns "no ticket available", the client may show an **elicitation** prompt: the server lists any stuck tickets (claimed or executing) and asks the user to confirm. The user can **direct the agent to claim** by choosing a ticket ID to force-release (e.g. agent-reliability-3); the server then force-releases that ticket and retries **claim_ticket** so the agent gets the lease in this session. Alternatively, the user can say "release (ticket_id) and claim it"—the agent should call **force_release_lease** (ticket_id) then **claim_ticket** (project_id). See **docs/mcp-human-in-the-loop.md** and **docs/troubleshooting.md** (releasing a stuck lease).

## Idempotency and retries

- **create_ticket**: Pass an optional **idempotency_key** (e.g. a UUID or deterministic key per "logical" create). Retries with the same key return the existing ticket instead of creating a duplicate. Use when the client may retry after timeouts or network errors.
- **claim_ticket**: Pass an optional **idempotency_key** (e.g. a stable key per "logical" claim, e.g. session or task ID). Retries with the same key: if the same agent still has a valid lease on the previously claimed ticket, the server renews and returns that ticket and lease; if the ticket is pending again (lease expired), the server re-claims that same ticket. Use so the same agent retrying after a timeout gets the same ticket instead of claiming another one.
- **Agent retry strategy**: Use idempotency keys for create and claim when operating in a retry-prone environment. For other tools, use **code** and **retriable** from error responses (see Error handling below).

## Error handling

Tool errors are returned as **JSON** in the error message. Parse the error string as JSON to get:

- **error** – Human-readable message.
- **code** – Stable code: lease_expired, unauthorized, forbidden, not_found, conflict, invalid_input, internal.
- **retriable** – If true, you may retry (e.g. lease_expired: try renew_lease or re-claim; not_found for "no ticket available": the client may prompt the user via elicitation, or try again later).

Use **code** to decide: lease_expired → renew or re-claim; unauthorized → ensure OAuth/sign-in; conflict → refresh ticket state; invalid_input → fix arguments. For the full list of codes and when to retry vs stop, see **docs/structured-errors.md**.

## Stuck tickets and runbook

If a ticket is **claimed** but not started (agent crashed), or you need to inspect ticket state and trace or release a stuck lease: see **docs/troubleshooting.md** → section **Tickets: agent stuck or wrong state**. Operators and agents can use that runbook to see how to inspect state (get_ticket, get_trace), when the lease expires and the ticket returns to pending, and how to force a ticket back to pending via REST (with or without the lease token).
`
