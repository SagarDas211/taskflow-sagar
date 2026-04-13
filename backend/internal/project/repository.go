package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrInvalidAssignee  = errors.New("invalid assignee")
	ErrInvalidTaskValue = errors.New("invalid task value")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateProject(ctx context.Context, value Project) (Project, error) {
	const query = `
		INSERT INTO projects (id, name, description, owner_id)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	err := r.db.QueryRow(ctx, query, value.ID, value.Name, value.Description, value.OwnerID).Scan(&value.CreatedAt)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}

	return value, nil
}

func (r *Repository) ListProjectsForUser(ctx context.Context, userID string, page, limit int) ([]Project, int, error) {
	const countQuery = `
		SELECT COUNT(DISTINCT p.id)
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		WHERE p.owner_id = $1 OR t.assignee_id = $1 OR t.creator_id = $1
	`

	var total int
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	const query = `
		SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		WHERE p.owner_id = $1 OR t.assignee_id = $1 OR t.creator_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var values []Project
	for rows.Next() {
		var value Project
		if err := rows.Scan(&value.ID, &value.Name, &value.Description, &value.OwnerID, &value.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}

	return values, total, nil
}

func (r *Repository) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)`, projectID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check project existence: %w", err)
	}
	return exists, nil
}

func (r *Repository) UserCanAccessProject(ctx context.Context, projectID, userID string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM projects p
			LEFT JOIN tasks t ON t.project_id = p.id
			WHERE p.id = $1
			  AND (p.owner_id = $2 OR t.assignee_id = $2 OR t.creator_id = $2)
		)
	`

	var allowed bool
	if err := r.db.QueryRow(ctx, query, projectID, userID).Scan(&allowed); err != nil {
		return false, fmt.Errorf("check project access: %w", err)
	}
	return allowed, nil
}

func (r *Repository) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	const query = `
		SELECT id, name, description, owner_id, created_at
		FROM projects
		WHERE id = $1
	`

	var value Project
	err := r.db.QueryRow(ctx, query, projectID).Scan(&value.ID, &value.Name, &value.Description, &value.OwnerID, &value.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return value, nil
}

func (r *Repository) UpdateProject(ctx context.Context, projectID string, input UpdateProjectInput) (Project, error) {
	const query = `
		UPDATE projects
		SET
			name = CASE WHEN $2 THEN $3 ELSE name END,
			description = CASE WHEN $4 THEN $5 ELSE description END
		WHERE id = $1
		RETURNING id, name, description, owner_id, created_at
	`

	var value Project
	err := r.db.QueryRow(ctx, query,
		projectID,
		input.Name.Set, input.Name.Value,
		input.Description.Set, input.Description.Value,
	).Scan(&value.ID, &value.Name, &value.Description, &value.OwnerID, &value.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("update project: %w", err)
	}
	return value, nil
}

func (r *Repository) DeleteProject(ctx context.Context, projectID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) IsProjectOwner(ctx context.Context, projectID, userID string) (bool, error) {
	var owner bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1 AND owner_id = $2)`, projectID, userID).Scan(&owner); err != nil {
		return false, fmt.Errorf("check project owner: %w", err)
	}
	return owner, nil
}

func (r *Repository) CreateTask(ctx context.Context, value Task) (Task, error) {
	const query = `
		INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, creator_id, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		value.ID, value.Title, value.Description, value.Status, value.Priority,
		value.ProjectID, value.AssigneeID, value.CreatorID, value.DueDate,
	).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		if isAssigneeForeignKeyViolation(err) {
			return Task{}, ErrInvalidAssignee
		}
		if isEnumOrUUIDError(err) {
			return Task{}, ErrInvalidTaskValue
		}
		return Task{}, fmt.Errorf("insert task: %w", err)
	}

	return value, nil
}

func (r *Repository) GetTaskByID(ctx context.Context, taskID string) (Task, error) {
	const query = `
		SELECT id, title, description, status::text, priority::text, project_id, assignee_id, creator_id, due_date, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	var value Task
	err := r.db.QueryRow(ctx, query, taskID).Scan(
		&value.ID, &value.Title, &value.Description, &value.Status, &value.Priority,
		&value.ProjectID, &value.AssigneeID, &value.CreatorID, &value.DueDate, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	return value, nil
}

func (r *Repository) ListTasksByProject(ctx context.Context, projectID string, filters TaskFilters) ([]Task, int, error) {
	where := []string{"project_id = $1"}
	args := []any{projectID}

	if filters.Status != "" {
		args = append(args, filters.Status)
		where = append(where, fmt.Sprintf("status = $%d::task_status", len(args)))
	}
	if filters.AssigneeID != "" {
		args = append(args, filters.AssigneeID)
		where = append(where, fmt.Sprintf("assignee_id = $%d", len(args)))
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, strings.Join(where, " AND "))
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	args = append(args, filters.Limit, (filters.Page-1)*filters.Limit)
	query := fmt.Sprintf(`
		SELECT id, title, description, status::text, priority::text, project_id, assignee_id, creator_id, due_date, created_at, updated_at
		FROM tasks
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var value Task
		if err := rows.Scan(
			&value.ID, &value.Title, &value.Description, &value.Status, &value.Priority,
			&value.ProjectID, &value.AssigneeID, &value.CreatorID, &value.DueDate, &value.CreatedAt, &value.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, total, nil
}

func (r *Repository) UpdateTask(ctx context.Context, taskID string, input UpdateTaskInput) (Task, error) {
	const query = `
		UPDATE tasks
		SET
			title = CASE WHEN $2 THEN $3 ELSE title END,
			description = CASE WHEN $4 THEN $5 ELSE description END,
			status = CASE WHEN $6 THEN $7::task_status ELSE status END,
			priority = CASE WHEN $8 THEN $9::task_priority ELSE priority END,
			assignee_id = CASE WHEN $10 THEN $11::uuid ELSE assignee_id END,
			due_date = CASE WHEN $12 THEN $13::date ELSE due_date END
		WHERE id = $1
		RETURNING id, title, description, status::text, priority::text, project_id, assignee_id, creator_id, due_date, created_at, updated_at
	`

	var dueDate any
	if input.DueDate.Value != nil {
		dueDate = input.DueDate.Value.Format("2006-01-02")
	}

	var value Task
	err := r.db.QueryRow(ctx, query,
		taskID,
		input.Title.Set, input.Title.Value,
		input.Description.Set, input.Description.Value,
		input.Status.Set, valueOrNil(input.Status.Value),
		input.Priority.Set, valueOrNil(input.Priority.Value),
		input.AssigneeID.Set, valueOrNil(input.AssigneeID.Value),
		input.DueDate.Set, dueDate,
	).Scan(
		&value.ID, &value.Title, &value.Description, &value.Status, &value.Priority,
		&value.ProjectID, &value.AssigneeID, &value.CreatorID, &value.DueDate, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		if isAssigneeForeignKeyViolation(err) {
			return Task{}, ErrInvalidAssignee
		}
		if isEnumOrUUIDError(err) {
			return Task{}, ErrInvalidTaskValue
		}
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	return value, nil
}

func (r *Repository) DeleteTask(ctx context.Context, taskID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) TaskDeleteAllowed(ctx context.Context, taskID, userID string) (Task, bool, error) {
	task, err := r.GetTaskByID(ctx, taskID)
	if err != nil {
		return Task{}, false, err
	}

	isOwner, err := r.IsProjectOwner(ctx, task.ProjectID, userID)
	if err != nil {
		return Task{}, false, err
	}
	if isOwner {
		return task, true, nil
	}
	if task.CreatorID != nil && *task.CreatorID == userID {
		return task, true, nil
	}
	return task, false, nil
}

func (r *Repository) ProjectStats(ctx context.Context, projectID string) (ProjectStats, error) {
	stats := ProjectStats{
		ByStatus:   map[string]int{},
		ByAssignee: map[string]int{},
	}

	statusRows, err := r.db.Query(ctx, `
		SELECT status::text, COUNT(*)
		FROM tasks
		WHERE project_id = $1
		GROUP BY status
	`, projectID)
	if err != nil {
		return ProjectStats{}, fmt.Errorf("stats by status: %w", err)
	}
	defer statusRows.Close()

	for statusRows.Next() {
		var name string
		var count int
		if err := statusRows.Scan(&name, &count); err != nil {
			return ProjectStats{}, fmt.Errorf("scan status stats: %w", err)
		}
		stats.ByStatus[name] = count
	}
	if err := statusRows.Err(); err != nil {
		return ProjectStats{}, fmt.Errorf("iterate status stats: %w", err)
	}

	assigneeRows, err := r.db.Query(ctx, `
		SELECT COALESCE(assignee_id::text, 'unassigned'), COUNT(*)
		FROM tasks
		WHERE project_id = $1
		GROUP BY assignee_id
	`, projectID)
	if err != nil {
		return ProjectStats{}, fmt.Errorf("stats by assignee: %w", err)
	}
	defer assigneeRows.Close()

	for assigneeRows.Next() {
		var name string
		var count int
		if err := assigneeRows.Scan(&name, &count); err != nil {
			return ProjectStats{}, fmt.Errorf("scan assignee stats: %w", err)
		}
		stats.ByAssignee[name] = count
	}
	if err := assigneeRows.Err(); err != nil {
		return ProjectStats{}, fmt.Errorf("iterate assignee stats: %w", err)
	}

	return stats, nil
}

func isAssigneeForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503" && strings.Contains(pgErr.ConstraintName, "assignee")
}

func isEnumOrUUIDError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "22P02"
}

func valueOrNil(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
