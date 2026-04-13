package project

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"taskflow/internal/cache"
)

const cacheTTL = 30 * time.Second

type Service struct {
	repo  *Repository
	cache cache.Cache
}

func NewService(repo *Repository, cacheProvider cache.Cache) *Service {
	return &Service{repo: repo, cache: cacheProvider}
}

func (s *Service) ListProjects(ctx context.Context, userID string, page, limit int) (PaginatedProjects, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	values, total, err := s.repo.ListProjectsForUser(ctx, userID, page, limit)
	if err != nil {
		return PaginatedProjects{}, err
	}

	return PaginatedProjects{Data: values, Page: page, Limit: limit, TotalCount: total}, nil
}

func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	value := Project{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(input.Name),
		Description: trimStringPointer(input.Description),
		OwnerID:     input.OwnerID,
	}

	created, err := s.repo.CreateProject(ctx, value)
	if err != nil {
		return Project{}, err
	}

	s.invalidateProjectCache(ctx, created.ID)
	return created, nil
}

func (s *Service) GetProjectDetails(ctx context.Context, projectID, userID string) (ProjectWithTasks, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allowed, err := s.repo.UserCanAccessProject(ctx, projectID, userID)
	if err != nil {
		return ProjectWithTasks{}, err
	}
	if !allowed {
		return ProjectWithTasks{}, s.ensureProjectError(ctx, projectID)
	}

	cacheKey := fmt.Sprintf("project:detail:%s", projectID)
	var cached ProjectWithTasks
	if found, err := s.cache.Get(ctx, cacheKey, &cached); err == nil && found {
		return cached, nil
	}

	value, err := s.repo.GetProjectByID(ctx, projectID)
	if err != nil {
		return ProjectWithTasks{}, err
	}

	tasks, _, err := s.repo.ListTasksByProject(ctx, projectID, TaskFilters{Page: 1, Limit: 1000})
	if err != nil {
		return ProjectWithTasks{}, err
	}

	response := ProjectWithTasks{Project: value, Tasks: tasks}
	_ = s.cache.Set(ctx, cacheKey, response, cacheTTL)
	return response, nil
}

func (s *Service) UpdateProject(ctx context.Context, projectID, userID string, input UpdateProjectInput) (Project, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isOwner, err := s.repo.IsProjectOwner(ctx, projectID, userID)
	if err != nil {
		return Project{}, err
	}
	if !isOwner {
		return Project{}, s.ensureProjectError(ctx, projectID)
	}

	input.Name.Value = trimStringPointer(input.Name.Value)
	input.Description.Value = trimStringPointer(input.Description.Value)

	value, err := s.repo.UpdateProject(ctx, projectID, input)
	if err != nil {
		return Project{}, err
	}

	s.invalidateProjectCache(ctx, projectID)
	return value, nil
}

func (s *Service) DeleteProject(ctx context.Context, projectID, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isOwner, err := s.repo.IsProjectOwner(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return s.ensureProjectError(ctx, projectID)
	}

	if err := s.repo.DeleteProject(ctx, projectID); err != nil {
		return err
	}

	s.invalidateProjectCache(ctx, projectID)
	return nil
}

func (s *Service) ListTasks(ctx context.Context, projectID, userID string, filters TaskFilters) (PaginatedTasks, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allowed, err := s.repo.UserCanAccessProject(ctx, projectID, userID)
	if err != nil {
		return PaginatedTasks{}, err
	}
	if !allowed {
		return PaginatedTasks{}, s.ensureProjectError(ctx, projectID)
	}

	values, total, err := s.repo.ListTasksByProject(ctx, projectID, filters)
	if err != nil {
		return PaginatedTasks{}, err
	}

	return PaginatedTasks{Data: values, Page: filters.Page, Limit: filters.Limit, TotalCount: total}, nil
}

func (s *Service) CreateTask(ctx context.Context, projectID, userID string, input CreateTaskInput) (Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allowed, err := s.repo.UserCanAccessProject(ctx, projectID, userID)
	if err != nil {
		return Task{}, err
	}
	if !allowed {
		return Task{}, s.ensureProjectError(ctx, projectID)
	}

	creatorID := userID
	value := Task{
		ID:          uuid.NewString(),
		Title:       strings.TrimSpace(input.Title),
		Description: trimStringPointer(input.Description),
		Status:      defaultString(strings.TrimSpace(input.Status), "todo"),
		Priority:    defaultString(strings.TrimSpace(input.Priority), "medium"),
		ProjectID:   projectID,
		AssigneeID:  trimStringPointer(input.AssigneeID),
		CreatorID:   &creatorID,
		DueDate:     input.DueDate,
	}

	created, err := s.repo.CreateTask(ctx, value)
	if err != nil {
		return Task{}, err
	}

	s.invalidateProjectCache(ctx, projectID)
	return created, nil
}

func (s *Service) UpdateTask(ctx context.Context, taskID, userID string, input UpdateTaskInput) (Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	allowed, err := s.repo.UserCanAccessProject(ctx, task.ProjectID, userID)
	if err != nil {
		return Task{}, err
	}
	if !allowed {
		return Task{}, ErrForbidden
	}

	input.Title.Value = trimStringPointer(input.Title.Value)
	input.Description.Value = trimStringPointer(input.Description.Value)
	input.AssigneeID.Value = trimStringPointer(input.AssigneeID.Value)
	if input.Status.Value != nil {
		trimmed := strings.TrimSpace(*input.Status.Value)
		input.Status.Value = &trimmed
	}
	if input.Priority.Value != nil {
		trimmed := strings.TrimSpace(*input.Priority.Value)
		input.Priority.Value = &trimmed
	}

	updated, err := s.repo.UpdateTask(ctx, taskID, input)
	if err != nil {
		return Task{}, err
	}

	s.invalidateProjectCache(ctx, task.ProjectID)
	return updated, nil
}

func (s *Service) DeleteTask(ctx context.Context, taskID, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	task, allowed, err := s.repo.TaskDeleteAllowed(ctx, taskID, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}

	if err := s.repo.DeleteTask(ctx, taskID); err != nil {
		return err
	}

	s.invalidateProjectCache(ctx, task.ProjectID)
	return nil
}

func (s *Service) Stats(ctx context.Context, projectID, userID string) (ProjectStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allowed, err := s.repo.UserCanAccessProject(ctx, projectID, userID)
	if err != nil {
		return ProjectStats{}, err
	}
	if !allowed {
		return ProjectStats{}, s.ensureProjectError(ctx, projectID)
	}

	cacheKey := fmt.Sprintf("project:stats:%s", projectID)
	var cached ProjectStats
	if found, err := s.cache.Get(ctx, cacheKey, &cached); err == nil && found {
		return cached, nil
	}

	stats, err := s.repo.ProjectStats(ctx, projectID)
	if err != nil {
		return ProjectStats{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, stats, cacheTTL)
	return stats, nil
}

func (s *Service) ensureProjectError(ctx context.Context, projectID string) error {
	exists, err := s.repo.ProjectExists(ctx, projectID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrForbidden
}

func (s *Service) invalidateProjectCache(ctx context.Context, projectID string) {
	_ = s.cache.Delete(ctx,
		fmt.Sprintf("project:detail:%s", projectID),
		fmt.Sprintf("project:stats:%s", projectID),
	)
}

func trimStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
