package project

import "time"

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	ProjectID   string     `json:"project_id"`
	AssigneeID  *string    `json:"assignee_id"`
	CreatorID   *string    `json:"creator_id"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ProjectWithTasks struct {
	Project Project `json:"project"`
	Tasks   []Task  `json:"tasks"`
}

type ProjectStats struct {
	ByStatus   map[string]int `json:"by_status"`
	ByAssignee map[string]int `json:"by_assignee"`
}

type PaginatedProjects struct {
	Data       []Project `json:"data"`
	Page       int       `json:"page"`
	Limit      int       `json:"limit"`
	TotalCount int       `json:"total_count"`
}

type PaginatedTasks struct {
	Data       []Task `json:"data"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	TotalCount int    `json:"total_count"`
}

type CreateProjectInput struct {
	Name        string
	Description *string
	OwnerID     string
}

type OptionalString struct {
	Set   bool
	Value *string
}

type OptionalTime struct {
	Set   bool
	Value *time.Time
}

type UpdateProjectInput struct {
	Name        OptionalString
	Description OptionalString
}

type CreateTaskInput struct {
	Title       string
	Description *string
	Status      string
	Priority    string
	AssigneeID  *string
	DueDate     *time.Time
	ProjectID   string
	CreatorID   string
}

type UpdateTaskInput struct {
	Title       OptionalString
	Description OptionalString
	Status      OptionalString
	Priority    OptionalString
	AssigneeID  OptionalString
	DueDate     OptionalTime
}

type TaskFilters struct {
	Status     string
	AssigneeID string
	Page       int
	Limit      int
}
