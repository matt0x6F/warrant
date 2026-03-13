-- users: GitHub identity, provisioned on first OAuth login
CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    github_id   BIGINT UNIQUE NOT NULL,
    login       TEXT NOT NULL,
    name        TEXT,
    email       TEXT,
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- link agent to user (one agent per user for API/MCP identity)
ALTER TABLE agents ADD COLUMN user_id TEXT REFERENCES users(id);
ALTER TABLE agents ALTER COLUMN api_key DROP NOT NULL;
CREATE UNIQUE INDEX agents_user_id_key ON agents(user_id) WHERE user_id IS NOT NULL;
