DROP TRIGGER IF EXISTS tasks_set_updated_at ON tasks;
DROP FUNCTION IF EXISTS set_tasks_updated_at();

DROP INDEX IF EXISTS tasks_priority_idx;
DROP INDEX IF EXISTS tasks_status_idx;
DROP INDEX IF EXISTS tasks_assignee_id_idx;
DROP INDEX IF EXISTS tasks_project_id_idx;
DROP TABLE IF EXISTS tasks;

DROP INDEX IF EXISTS projects_owner_id_idx;
DROP TABLE IF EXISTS projects;

DROP INDEX IF EXISTS users_email_unique_idx;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;
