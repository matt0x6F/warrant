ALTER TABLE tickets DROP COLUMN IF EXISTS work_stream_id;
DROP INDEX IF EXISTS tickets_work_stream;
DROP TABLE IF EXISTS work_streams;
