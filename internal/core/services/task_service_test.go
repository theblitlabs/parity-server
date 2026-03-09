package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type inMemoryTaskRepo struct {
	tasks   map[uuid.UUID]*models.Task
	results map[uuid.UUID]*models.TaskResult
}

func newInMemoryTaskRepo() *inMemoryTaskRepo {
	return &inMemoryTaskRepo{
		tasks:   make(map[uuid.UUID]*models.Task),
		results: make(map[uuid.UUID]*models.TaskResult),
	}
}

func (r *inMemoryTaskRepo) Create(ctx context.Context, task *models.Task) error {
	r.tasks[task.ID] = cloneTask(task)
	return nil
}

func (r *inMemoryTaskRepo) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return cloneTask(task), nil
}

func (r *inMemoryTaskRepo) Update(ctx context.Context, task *models.Task) error {
	r.tasks[task.ID] = cloneTask(task)
	return nil
}

func (r *inMemoryTaskRepo) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	tasks := make([]*models.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, cloneTask(task))
	}
	return tasks, nil
}

func (r *inMemoryTaskRepo) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	tasks := make([]*models.Task, 0)
	for _, task := range r.tasks {
		if task.Status == status {
			tasks = append(tasks, cloneTask(task))
		}
	}
	return tasks, nil
}

func (r *inMemoryTaskRepo) GetAll(ctx context.Context) ([]models.Task, error) {
	return nil, nil
}

func (r *inMemoryTaskRepo) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	cloned := *result
	r.results[result.TaskID] = &cloned
	return nil
}

func (r *inMemoryTaskRepo) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	result, ok := r.results[taskID]
	if !ok {
		return nil, nil
	}
	cloned := *result
	return &cloned, nil
}

func (r *inMemoryTaskRepo) GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error) {
	return nil, nil
}

type inMemoryRunnerRepo struct {
	runners map[string]*models.Runner
}

func newInMemoryRunnerRepo() *inMemoryRunnerRepo {
	return &inMemoryRunnerRepo{
		runners: make(map[string]*models.Runner),
	}
}

func (r *inMemoryRunnerRepo) Create(ctx context.Context, runner *models.Runner) error {
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return nil
}

func (r *inMemoryRunnerRepo) Get(ctx context.Context, deviceID string) (*models.Runner, error) {
	runner, ok := r.runners[deviceID]
	if !ok {
		return nil, ErrRunnerNotFound
	}
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) CreateOrUpdate(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) Update(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	runners := make([]*models.Runner, 0)
	for _, runner := range r.runners {
		if runner.Status == status {
			runners = append(runners, cloneRunner(runner))
		}
	}
	return runners, nil
}

func (r *inMemoryRunnerRepo) UpdateRunnersToOffline(ctx context.Context, heartbeatTimeout time.Duration) (int64, []string, error) {
	return 0, nil, nil
}

func (r *inMemoryRunnerRepo) GetOnlineRunners(ctx context.Context) ([]*models.Runner, error) {
	return nil, nil
}

func (r *inMemoryRunnerRepo) GetRunnerByDeviceID(ctx context.Context, deviceID string) (*models.Runner, error) {
	return r.Get(ctx, deviceID)
}

func (r *inMemoryRunnerRepo) UpdateModelCapabilities(ctx context.Context, runnerID string, capabilities []models.ModelCapability) error {
	return nil
}

func cloneTask(task *models.Task) *models.Task {
	cloned := *task
	if task.Config != nil {
		cloned.Config = append([]byte(nil), task.Config...)
	}
	if task.Environment != nil {
		envCopy := *task.Environment
		if task.Environment.Config != nil {
			envCopy.Config = make(map[string]interface{}, len(task.Environment.Config))
			for key, value := range task.Environment.Config {
				envCopy.Config[key] = value
			}
		}
		cloned.Environment = &envCopy
	}
	return &cloned
}

func cloneRunner(runner *models.Runner) *models.Runner {
	cloned := *runner
	if runner.TaskID != nil {
		taskID := *runner.TaskID
		cloned.TaskID = &taskID
	}
	return &cloned
}

func TestAssignTaskToRunnerRevertsStateWhenNotificationFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "runner unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "hello"
	task.Description = "test"
	task.Type = models.TaskTypeDocker
	task.Nonce = "original-nonce"
	task.Environment = &models.EnvironmentConfig{
		Type: "docker",
		Config: map[string]interface{}{
			"workdir": "/",
		},
	}

	config, err := json.Marshal(models.TaskConfig{
		ImageName: "hello-world:latest",
	})
	if err != nil {
		t.Fatalf("failed to marshal task config: %v", err)
	}
	task.Config = config
	taskRepo.tasks[task.ID] = cloneTask(task)

	runner := &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		Webhook:  server.URL,
	}
	runnerRepo.runners[runner.DeviceID] = cloneRunner(runner)

	err = taskService.assignTaskToRunner(context.Background(), task, runner)
	if err == nil {
		t.Fatal("expected assignment to fail when webhook notification fails")
	}

	storedTask, getErr := taskRepo.Get(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("failed to get stored task: %v", getErr)
	}

	if storedTask.RunnerID != "" {
		t.Fatalf("expected task runner assignment to be reverted, got %q", storedTask.RunnerID)
	}

	if storedTask.Nonce != "original-nonce" {
		t.Fatalf("expected task nonce to be reverted, got %q", storedTask.Nonce)
	}

	storedRunner, getErr := runnerRepo.Get(context.Background(), runner.DeviceID)
	if getErr != nil {
		t.Fatalf("failed to get stored runner: %v", getErr)
	}

	if storedRunner.TaskID != nil {
		t.Fatalf("expected runner task assignment to be cleared, got %s", storedRunner.TaskID.String())
	}
}

func TestAssignTaskToRunnerRejectsBusyRunnerUsingFreshState(t *testing.T) {
	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "queued"
	task.Description = "pending task"
	task.Type = models.TaskTypeDocker
	task.Nonce = "pending-nonce"
	task.Status = models.TaskStatusPending
	task.Environment = &models.EnvironmentConfig{
		Type: "docker",
		Config: map[string]interface{}{
			"workdir": "/",
		},
	}
	config, err := json.Marshal(models.TaskConfig{ImageName: "hello-world:latest"})
	if err != nil {
		t.Fatalf("failed to marshal task config: %v", err)
	}
	task.Config = config
	taskRepo.tasks[task.ID] = cloneTask(task)

	otherTaskID := uuid.New()
	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		TaskID:   &otherTaskID,
		Webhook:  "http://runner.invalid/webhook",
	}

	// Simulate a stale runner snapshot captured before another assignment landed.
	staleRunner := &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		Webhook:  "http://runner.invalid/webhook",
		TaskID:   nil,
	}

	err = taskService.assignTaskToRunner(context.Background(), task, staleRunner)
	if err == nil {
		t.Fatal("expected assignment to fail for busy runner")
	}

	storedTask, getErr := taskRepo.Get(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("failed to get stored task: %v", getErr)
	}
	if storedTask.RunnerID != "" {
		t.Fatalf("expected task to remain unassigned, got %q", storedTask.RunnerID)
	}

	storedRunner, getErr := runnerRepo.Get(context.Background(), "runner-1")
	if getErr != nil {
		t.Fatalf("failed to get stored runner: %v", getErr)
	}
	if storedRunner.TaskID == nil || *storedRunner.TaskID != otherTaskID {
		t.Fatalf("expected runner task assignment to remain %s", otherTaskID.String())
	}
}

func TestSaveTaskResultMarksVerifiedWhenHashesMatch(t *testing.T) {
	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "verified"
	task.Description = "hash match"
	task.Type = models.TaskTypeDocker
	task.Status = models.TaskStatusRunning
	task.RunnerID = "runner-1"
	task.ImageHash = "image-hash"
	task.CommandHash = "command-hash"
	taskRepo.tasks[task.ID] = cloneTask(task)

	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusBusy,
		TaskID:   &task.ID,
	}

	result := models.NewTaskResult()
	result.TaskID = task.ID
	result.DeviceID = "runner-1"
	result.SolverDeviceID = "runner-1"
	result.ImageHashVerified = "image-hash"
	result.CommandHashVerified = "command-hash"

	if err := taskService.SaveTaskResult(context.Background(), result); err != nil {
		t.Fatalf("SaveTaskResult() error = %v", err)
	}

	storedResult, err := taskRepo.GetTaskResult(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTaskResult() error = %v", err)
	}

	if storedResult.VerificationStatus != "verified" {
		t.Fatalf("verification status = %q, want %q", storedResult.VerificationStatus, "verified")
	}

	storedTask, err := taskRepo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if storedTask.Status != models.TaskStatusCompleted {
		t.Fatalf("task status = %q, want %q", storedTask.Status, models.TaskStatusCompleted)
	}
}

func TestSaveTaskResultMarksNotVerifiedWhenHashesMismatch(t *testing.T) {
	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "mismatch"
	task.Description = "hash mismatch"
	task.Type = models.TaskTypeDocker
	task.Status = models.TaskStatusRunning
	task.RunnerID = "runner-1"
	task.ImageHash = "expected-image"
	taskRepo.tasks[task.ID] = cloneTask(task)

	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusBusy,
		TaskID:   &task.ID,
	}

	result := models.NewTaskResult()
	result.TaskID = task.ID
	result.DeviceID = "runner-1"
	result.SolverDeviceID = "runner-1"
	result.ImageHashVerified = "different-image"

	if err := taskService.SaveTaskResult(context.Background(), result); err != nil {
		t.Fatalf("SaveTaskResult() error = %v", err)
	}

	storedResult, err := taskRepo.GetTaskResult(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTaskResult() error = %v", err)
	}

	if storedResult.VerificationStatus != "failed" {
		t.Fatalf("verification status = %q, want %q", storedResult.VerificationStatus, "failed")
	}

	storedTask, err := taskRepo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if storedTask.Status != models.TaskStatusNotVerified {
		t.Fatalf("task status = %q, want %q", storedTask.Status, models.TaskStatusNotVerified)
	}
}

func TestCheckPendingAssignmentsResetsStaleAssignedTask(t *testing.T) {
	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "stale"
	task.Description = "assigned but never started"
	task.Type = models.TaskTypeDocker
	task.Status = models.TaskStatusPending
	task.RunnerID = "runner-1"
	task.UpdatedAt = time.Now().Add(-pendingAssignmentTimeout - time.Second)
	taskRepo.tasks[task.ID] = cloneTask(task)

	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		TaskID:   &task.ID,
	}

	if err := taskService.checkPendingAssignments(); err != nil {
		t.Fatalf("checkPendingAssignments() error = %v", err)
	}

	storedTask, err := taskRepo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if storedTask.RunnerID != "" {
		t.Fatalf("expected stale task assignment to be cleared, got %q", storedTask.RunnerID)
	}
	if storedTask.Status != models.TaskStatusPending {
		t.Fatalf("task status = %q, want %q", storedTask.Status, models.TaskStatusPending)
	}

	storedRunner, err := runnerRepo.Get(context.Background(), "runner-1")
	if err != nil {
		t.Fatalf("Get runner error = %v", err)
	}
	if storedRunner.TaskID != nil {
		t.Fatalf("expected runner task ID to be cleared, got %s", storedRunner.TaskID.String())
	}
}
