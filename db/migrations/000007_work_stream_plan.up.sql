-- work stream long-form text is an implementation plan (Markdown), not a short description
ALTER TABLE work_streams RENAME COLUMN description TO plan;
