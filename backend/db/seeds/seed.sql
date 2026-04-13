BEGIN;

TRUNCATE TABLE tasks, projects, users RESTART IDENTITY CASCADE;

INSERT INTO users (id, name, email, password, created_at)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Test User',
    'test@example.com',
    '$2a$12$BCgl8jZR6IZ0tD//xBXFTOC9bJLP6D.QmAbpPIZrjzZrXwQ95VnOW',
    NOW()
);

INSERT INTO projects (id, name, description, owner_id, created_at)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    'Seed Project',
    'Project created by the seed script for reviewer testing.',
    '11111111-1111-1111-1111-111111111111',
    NOW()
);

INSERT INTO tasks (
    id,
    title,
    description,
    status,
    priority,
    project_id,
    assignee_id,
    creator_id,
    due_date,
    created_at,
    updated_at
)
VALUES
(
    '33333333-3333-3333-3333-333333333331',
    'Seed Task Todo',
    'Task in todo state.',
    'todo',
    'high',
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    '11111111-1111-1111-1111-111111111111',
    CURRENT_DATE + INTERVAL '3 days',
    NOW(),
    NOW()
),
(
    '33333333-3333-3333-3333-333333333332',
    'Seed Task In Progress',
    'Task in progress state.',
    'in_progress',
    'medium',
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    '11111111-1111-1111-1111-111111111111',
    CURRENT_DATE + INTERVAL '5 days',
    NOW(),
    NOW()
),
(
    '33333333-3333-3333-3333-333333333333',
    'Seed Task Done',
    'Task in done state.',
    'done',
    'low',
    '22222222-2222-2222-2222-222222222222',
    NULL,
    '11111111-1111-1111-1111-111111111111',
    CURRENT_DATE + INTERVAL '7 days',
    NOW(),
    NOW()
);

COMMIT;
