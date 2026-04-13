# Task Flow

## 1. Overview
Task Flow is a backend-only task management API built with Go, Gin, PostgreSQL, Redis, and Docker Compose.

Repo structure:

```text
.
├── backend/
│   ├── cmd/
│   ├── db/
│   ├── internal/
│   ├── scripts/
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── docker-compose.yml
├── .env.example
└── README.md
```

It provides:
- JWT-based authentication
- project CRUD with owner authorization
- task CRUD with filtering, pagination, and stats
- PostgreSQL migrations managed explicitly with SQL files
- Redis-backed caching for selected read-heavy endpoints

Tech stack:
- Go 1.25
- Gin
- PostgreSQL 17
- Redis 7
- `golang-migrate`
- Docker Compose

## 2. Architecture Decisions
I split the backend into small packages so HTTP handlers, business logic, database queries, config, and caching stay reviewable and do not turn into god functions.

Main decisions:
- `internal/httpapi`: request parsing, validation, status codes, JSON responses
- `internal/auth`: registration, login, bcrypt hashing, JWT creation/parsing
- `internal/project`: project/task business rules and SQL repository logic
- `internal/cache`: Redis abstraction with a no-op fallback
- `db/migrations`: explicit SQL migrations with both `up` and `down`

Tradeoffs:
- I used plain SQL instead of an ORM so the schema and queries stay explicit and migration-friendly.
- Redis is used only for a few read paths right now instead of aggressive caching everywhere, which keeps invalidation simpler.
- The API is backend-only; I intentionally left out a frontend because this submission is focused on the backend rubric.

Intentionally left out:
- refresh tokens
- rate limiting
- background jobs
- OpenAPI/Postman export
- advanced observability beyond structured logs

Those are good next steps, but I prioritized correctness, Docker usability, explicit migrations, and reviewer-friendly test flows first.

## 3. Running Locally
Assume only Docker and Docker Compose are installed.

```bash
git clone https://github.com/your-name/taskflow
cd taskflow
cp .env.example .env
docker compose up --build
```

Backend available at:

```text
http://localhost:8080
```

To seed review data in a second terminal:

```bash
./backend/scripts/seed-db.sh
```

## 4. Running Migrations
Migrations run automatically when the API container starts.

That means this command already applies all `up` migrations before the API begins serving traffic:

```bash
docker compose up --build
```

If you want to run migrations manually outside Docker:

```bash
cd backend
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction down
go run ./cmd/migrate -direction down -steps 2
```

Every migration file in `db/migrations/` has both `up` and `down` files:
- `backend/db/migrations/000001_init.up.sql`
- `backend/db/migrations/000001_init.down.sql`
- `backend/db/migrations/000002_add_task_creator.up.sql`
- `backend/db/migrations/000002_add_task_creator.down.sql`

## 5. Test Credentials
After running:

```bash
./backend/scripts/seed-db.sh
```

Use:

```text
Email:    test@example.com
Password: password123
```

The seeded password is stored as a bcrypt hash in the database, not plaintext.

## 6. API Reference
Base URL:

```text
http://localhost:8080
```

Auth endpoints:

### `POST /auth/register`
Request:

```json
{
  "name": "Sagar",
  "email": "sagar@example.com",
  "password": "Sagar@1234"
}
```

Response `201`:

```json
{
  "id": "uuid",
  "name": "Sagar",
  "email": "sagar@example.com",
  "created_at": "2026-04-13T10:00:00Z"
}
```

### `POST /auth/login`
Request:

```json
{
  "email": "sagar@example.com",
  "password": "Sagar@1234"
}
```

Response `200`:

```json
{
  "access_token": "jwt-token"
}
```

### `GET /auth/me`
Requires `Authorization: Bearer <token>`

Response `200`:

```json
{
  "user_id": "uuid",
  "email": "sagar@example.com"
}
```

Project endpoints:

### `GET /projects?page=1&limit=10`
Lists projects the current user owns or has tasks in.

### `POST /projects`
Request:

```json
{
  "name": "Project 1",
  "description": "Backend review project"
}
```

### `GET /projects/:id`
Returns project details plus its tasks.

### `PATCH /projects/:id`
Owner only.

Request:

```json
{
  "name": "Updated Project Name",
  "description": "Updated description"
}
```

### `DELETE /projects/:id`
Owner only. Deletes the project and all its tasks.

Task endpoints:

### `GET /projects/:id/tasks?status=todo&assignee=<user_id>&page=1&limit=10`
Lists tasks in a project with optional filters and pagination.

### `POST /projects/:id/tasks`
Request:

```json
{
  "title": "Task 1",
  "description": "Do something important",
  "status": "todo",
  "priority": "high",
  "assignee_id": "uuid",
  "due_date": "2026-05-01"
}
```

### `PATCH /tasks/:id`
Request:

```json
{
  "title": "Refined task title",
  "status": "in_progress",
  "priority": "medium",
  "assignee_id": "uuid",
  "due_date": "2026-05-03"
}
```

### `DELETE /tasks/:id`
Allowed for project owner or task creator only.

### `GET /projects/:id/stats`
Response `200`:

```json
{
  "by_status": {
    "todo": 1,
    "in_progress": 1,
    "done": 1
  },
  "by_assignee": {
    "unassigned": 1,
    "11111111-1111-1111-1111-111111111111": 2
  }
}
```

Common error responses:

Validation error `400`

```json
{
  "error": "validation failed",
  "fields": {
    "email": "is required"
  }
}
```

Unauthenticated `401`

```json
{
  "error": "unauthenticated"
}
```

Forbidden `403`

```json
{
  "error": "forbidden"
}
```

Not found `404`

```json
{
  "error": "not found"
}
```

## 7. What You'd Do With More Time
If I had more time, I would improve a few areas:

- add a proper OpenAPI spec and importable Postman/Bruno collection
- expand test coverage for edge cases like invalid UUIDs, bad date formats, and cache invalidation behavior
- add refresh tokens and logout/token revocation support
- add request IDs, metrics, and tracing
- harden Docker for production with non-root runtime user and stricter health/readiness semantics
- add a dedicated seed container or make seeding optional through Compose profiles

Shortcuts I took:
- caching is intentionally limited to a few read endpoints instead of a broad caching strategy
- integration tests use the real database but do not yet run inside Docker as a dedicated test service
- README examples are concise instead of exhaustive for every single response variation
