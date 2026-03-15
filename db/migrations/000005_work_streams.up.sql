-- work_streams: logical grouping of tickets toward a goal (e.g. "Productionize feature A")
CREATE TABLE work_streams (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    description TEXT,
    branch      TEXT,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

ALTER TABLE work_streams ADD CONSTRAINT work_streams_status_check CHECK (status IN ('active', 'closed'));

CREATE INDEX work_streams_project_status ON work_streams(project_id, status);

-- tickets can optionally belong to a work stream
ALTER TABLE tickets ADD COLUMN work_stream_id TEXT REFERENCES work_streams(id);
CREATE INDEX tickets_work_stream ON tickets(work_stream_id);
