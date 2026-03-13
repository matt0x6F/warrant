-- Idempotency for create_ticket: same (project_id, idempotency_key) returns existing ticket.
CREATE TABLE idempotency_creates (
    project_id      TEXT NOT NULL REFERENCES projects(id),
    idempotency_key TEXT NOT NULL,
    ticket_id       TEXT NOT NULL REFERENCES tickets(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, idempotency_key)
);
