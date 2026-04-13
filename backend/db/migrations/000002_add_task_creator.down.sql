DROP INDEX IF EXISTS tasks_creator_id_idx;

ALTER TABLE tasks
DROP COLUMN IF EXISTS creator_id;
