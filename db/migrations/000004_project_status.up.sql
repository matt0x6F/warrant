-- Project status: active (default) or closed. List endpoints default to active only.
ALTER TABLE projects ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE projects ADD CONSTRAINT projects_status_check CHECK (status IN ('active', 'closed'));
