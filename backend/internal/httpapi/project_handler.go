package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"taskflow/internal/project"
)

type ProjectHandler struct {
	service *project.Service
}

type createProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type createTaskRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

func NewProjectHandler(service *project.Service) *ProjectHandler {
	return &ProjectHandler{service: service}
}

func (h *ProjectHandler) ListProjects(c *gin.Context) {
	page, limit, fields := parsePagination(c)
	if len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	response, err := h.service.ListProjects(c.Request.Context(), c.GetString("user_id"), page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var request createProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	if strings.TrimSpace(request.Name) == "" {
		respondValidationError(c, map[string]string{"name": "is required"})
		return
	}

	response, err := h.service.CreateProject(c.Request.Context(), project.CreateProjectInput{
		Name:        request.Name,
		Description: request.Description,
		OwnerID:     c.GetString("user_id"),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *ProjectHandler) GetProject(c *gin.Context) {
	response, err := h.service.GetProjectDetails(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	var body map[string]json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	input := project.UpdateProjectInput{}
	fields := map[string]string{}

	if raw, ok := body["name"]; ok {
		input.Name.Set = true
		if string(raw) == "null" {
			fields["name"] = "cannot be null"
		} else {
			var name string
			if err := json.Unmarshal(raw, &name); err != nil {
				fields["name"] = "must be a string"
			} else if strings.TrimSpace(name) == "" {
				fields["name"] = "cannot be blank"
			} else {
				input.Name.Value = &name
			}
		}
	}

	if raw, ok := body["description"]; ok {
		input.Description.Set = true
		if string(raw) != "null" {
			var description string
			if err := json.Unmarshal(raw, &description); err != nil {
				fields["description"] = "must be a string or null"
			} else {
				input.Description.Value = &description
			}
		}
	}

	if len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	response, err := h.service.UpdateProject(c.Request.Context(), c.Param("id"), c.GetString("user_id"), input)
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	if err := h.service.DeleteProject(c.Request.Context(), c.Param("id"), c.GetString("user_id")); err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
}

func (h *ProjectHandler) ListTasks(c *gin.Context) {
	page, limit, fields := parsePagination(c)
	if len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !validStatus(status) {
		respondValidationError(c, map[string]string{"status": "must be one of todo, in_progress, done"})
		return
	}

	response, err := h.service.ListTasks(c.Request.Context(), c.Param("id"), c.GetString("user_id"), project.TaskFilters{
		Status:     status,
		AssigneeID: strings.TrimSpace(c.Query("assignee")),
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) CreateTask(c *gin.Context) {
	var request createTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	fields := map[string]string{}
	if strings.TrimSpace(request.Title) == "" {
		fields["title"] = "is required"
	}
	if request.Status != "" && !validStatus(request.Status) {
		fields["status"] = "must be one of todo, in_progress, done"
	}
	if request.Priority != "" && !validPriority(request.Priority) {
		fields["priority"] = "must be one of low, medium, high"
	}

	dueDate, dateFields := parseOptionalDate(request.DueDate)
	for key, value := range dateFields {
		fields[key] = value
	}
	if len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	response, err := h.service.CreateTask(c.Request.Context(), c.Param("id"), c.GetString("user_id"), project.CreateTaskInput{
		Title:       request.Title,
		Description: request.Description,
		Status:      request.Status,
		Priority:    request.Priority,
		AssigneeID:  request.AssigneeID,
		DueDate:     dueDate,
	})
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *ProjectHandler) UpdateTask(c *gin.Context) {
	var body map[string]json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	input := project.UpdateTaskInput{}
	fields := map[string]string{}

	parseOptionalStringField(body, "title", &input.Title, fields, true)
	parseOptionalStringField(body, "description", &input.Description, fields, false)
	parseOptionalStringField(body, "status", &input.Status, fields, false)
	parseOptionalStringField(body, "priority", &input.Priority, fields, false)
	parseOptionalStringField(body, "assignee_id", &input.AssigneeID, fields, false)
	parseOptionalDateField(body, "due_date", &input.DueDate, fields)

	if input.Status.Set && input.Status.Value != nil && !validStatus(*input.Status.Value) {
		fields["status"] = "must be one of todo, in_progress, done"
	}
	if input.Priority.Set && input.Priority.Value != nil && !validPriority(*input.Priority.Value) {
		fields["priority"] = "must be one of low, medium, high"
	}

	if len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	response, err := h.service.UpdateTask(c.Request.Context(), c.Param("id"), c.GetString("user_id"), input)
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) DeleteTask(c *gin.Context) {
	if err := h.service.DeleteTask(c.Request.Context(), c.Param("id"), c.GetString("user_id")); err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}

func (h *ProjectHandler) Stats(c *gin.Context) {
	response, err := h.service.Stats(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	if err != nil {
		handleProjectError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func handleProjectError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		respondError(c, http.StatusNotFound, "not found")
	case errors.Is(err, project.ErrForbidden):
		respondError(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, project.ErrInvalidAssignee):
		respondValidationError(c, map[string]string{"assignee_id": "is invalid"})
	case errors.Is(err, project.ErrInvalidTaskValue):
		respondValidationError(c, map[string]string{"task": "contains invalid values"})
	default:
		respondError(c, http.StatusInternalServerError, "internal server error")
	}
}

func parsePagination(c *gin.Context) (int, int, map[string]string) {
	page := 1
	limit := 10
	fields := map[string]string{}

	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			fields["page"] = "must be a positive integer"
		} else {
			page = parsed
		}
	}

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			fields["limit"] = "must be between 1 and 100"
		} else {
			limit = parsed
		}
	}

	return page, limit, fields
}

func validStatus(value string) bool {
	switch value {
	case "todo", "in_progress", "done":
		return true
	default:
		return false
	}
}

func validPriority(value string) bool {
	switch value {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func parseOptionalDate(value *string) (*time.Time, map[string]string) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, map[string]string{"due_date": "must be in YYYY-MM-DD format"}
	}

	return &parsed, nil
}

func parseOptionalStringField(body map[string]json.RawMessage, key string, target *project.OptionalString, fields map[string]string, rejectNull bool) {
	raw, ok := body[key]
	if !ok {
		return
	}

	target.Set = true
	if string(raw) == "null" {
		if rejectNull {
			fields[key] = "cannot be null"
		}
		return
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		fields[key] = "must be a string"
		return
	}

	if rejectNull && strings.TrimSpace(value) == "" {
		fields[key] = "cannot be blank"
		return
	}

	target.Value = &value
}

func parseOptionalDateField(body map[string]json.RawMessage, key string, target *project.OptionalTime, fields map[string]string) {
	raw, ok := body[key]
	if !ok {
		return
	}

	target.Set = true
	if string(raw) == "null" {
		return
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		fields[key] = "must be a date string or null"
		return
	}

	if strings.TrimSpace(value) == "" {
		return
	}

	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		fields[key] = "must be in YYYY-MM-DD format"
		return
	}

	target.Value = &parsed
}
