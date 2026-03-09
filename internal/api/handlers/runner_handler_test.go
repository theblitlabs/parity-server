package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

func TestListAvailableTasksFiltersAssignmentsForRunner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &runnerHandlerTaskRepo{
		tasks: make(map[uuid.UUID]*models.Task),
	}
	taskService := services.NewTaskService(repo, nil, nil)
	handler := NewRunnerHandler(taskService, nil)

	unassigned := models.NewTask()
	unassigned.Title = "unassigned"
	unassigned.Description = "visible"
	unassigned.Type = models.TaskTypeDocker
	unassigned.Status = models.TaskStatusPending

	assignedToCaller := models.NewTask()
	assignedToCaller.Title = "assigned-to-caller"
	assignedToCaller.Description = "visible"
	assignedToCaller.Type = models.TaskTypeDocker
	assignedToCaller.Status = models.TaskStatusPending
	assignedToCaller.RunnerID = "runner-1"

	assignedToOther := models.NewTask()
	assignedToOther.Title = "assigned-to-other"
	assignedToOther.Description = "hidden"
	assignedToOther.Type = models.TaskTypeDocker
	assignedToOther.Status = models.TaskStatusPending
	assignedToOther.RunnerID = "runner-2"

	repo.tasks[unassigned.ID] = cloneHandlerTask(unassigned)
	repo.tasks[assignedToCaller.ID] = cloneHandlerTask(assignedToCaller)
	repo.tasks[assignedToOther.ID] = cloneHandlerTask(assignedToOther)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runners/tasks/available", nil)
	req.Header.Set("X-Device-ID", "runner-1")
	rec := httptest.NewRecorder()

	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	handler.ListAvailableTasks(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var tasks []*models.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &tasks); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("task count = %d, want 2", len(tasks))
	}

	for _, task := range tasks {
		if task.RunnerID != "" && task.RunnerID != "runner-1" {
			t.Fatalf("unexpected task for other runner returned: %s", task.ID.String())
		}
	}
}

type runnerHandlerTaskRepo struct {
	tasks map[uuid.UUID]*models.Task
}

func (r *runnerHandlerTaskRepo) Create(ctx context.Context, task *models.Task) error {
	r.tasks[task.ID] = cloneHandlerTask(task)
	return nil
}

func (r *runnerHandlerTaskRepo) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, services.ErrTaskNotFound
	}
	return cloneHandlerTask(task), nil
}

func (r *runnerHandlerTaskRepo) Update(ctx context.Context, task *models.Task) error {
	r.tasks[task.ID] = cloneHandlerTask(task)
	return nil
}

func (r *runnerHandlerTaskRepo) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	return nil, nil
}

func (r *runnerHandlerTaskRepo) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	tasks := make([]*models.Task, 0)
	for _, task := range r.tasks {
		if task.Status == status {
			tasks = append(tasks, cloneHandlerTask(task))
		}
	}
	return tasks, nil
}

func (r *runnerHandlerTaskRepo) GetAll(ctx context.Context) ([]models.Task, error) {
	return nil, nil
}

func (r *runnerHandlerTaskRepo) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	return nil
}

func (r *runnerHandlerTaskRepo) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	return nil, nil
}

func (r *runnerHandlerTaskRepo) GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error) {
	return nil, nil
}

func cloneHandlerTask(task *models.Task) *models.Task {
	cloned := *task
	if task.CompletedAt != nil {
		completedAt := *task.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	cloned.CreatedAt = task.CreatedAt
	cloned.UpdatedAt = task.UpdatedAt
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now()
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	return &cloned
}
