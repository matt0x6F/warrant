-- orgs
CREATE TABLE orgs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE org_members (
    org_id      TEXT REFERENCES orgs(id),
    user_id     TEXT NOT NULL,
    role        TEXT NOT NULL,
    PRIMARY KEY (org_id, user_id)
);

-- projects
CREATE TABLE projects (
    id              TEXT PRIMARY KEY,
    org_id          TEXT REFERENCES orgs(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    repo_url        TEXT,
    tech_stack      TEXT[],
    context_pack    JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

-- agents (client identities: IDE, Claude Teams, CI, etc.)
CREATE TABLE agents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    api_key     TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- per-project ticket sequence for IDs like <project-slug>-<seq>
CREATE TABLE ticket_sequences (
    project_id  TEXT PRIMARY KEY REFERENCES projects(id),
    next_val    BIGINT NOT NULL DEFAULT 0
);

-- tickets
CREATE TABLE tickets (
    id              TEXT PRIMARY KEY,
    project_id      TEXT REFERENCES projects(id),
    title           TEXT NOT NULL,
    type            TEXT NOT NULL,
    priority        INT NOT NULL DEFAULT 2,
    state           TEXT NOT NULL DEFAULT 'pending',
    version         INT NOT NULL DEFAULT 0,
    objective       JSONB NOT NULL,
    ticket_context  JSONB NOT NULL DEFAULT '{}',
    inputs          JSONB NOT NULL DEFAULT '{}',
    outputs         JSONB NOT NULL DEFAULT '{}',
    depends_on      TEXT[] NOT NULL DEFAULT '{}',
    assigned_to     TEXT REFERENCES agents(id),
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tickets_project_state ON tickets(project_id, state);
CREATE INDEX tickets_state_priority ON tickets(state, priority);

-- execution traces
CREATE TABLE execution_steps (
    id          TEXT PRIMARY KEY,
    ticket_id   TEXT REFERENCES tickets(id),
    agent_id    TEXT REFERENCES agents(id),
    type        TEXT NOT NULL,
    payload     JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- reviews
CREATE TABLE reviews (
    id          TEXT PRIMARY KEY,
    ticket_id   TEXT REFERENCES tickets(id),
    reviewer_id TEXT NOT NULL,
    decision    TEXT NOT NULL,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- escalations
CREATE TABLE escalations (
    id          TEXT PRIMARY KEY,
    ticket_id   TEXT REFERENCES tickets(id),
    agent_id    TEXT REFERENCES agents(id),
    reason      TEXT NOT NULL,
    question    TEXT NOT NULL,
    answer      TEXT,
    resolved_by TEXT,
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
