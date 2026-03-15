-- default_branch: branch to checkout when closing a work stream (e.g. main, master)
ALTER TABLE projects ADD COLUMN default_branch TEXT NOT NULL DEFAULT 'main';
