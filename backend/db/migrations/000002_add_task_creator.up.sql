ALTER TABLE tasks
ADD COLUMN creator_id UUID REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX tasks_creator_id_idx ON tasks (creator_id);
