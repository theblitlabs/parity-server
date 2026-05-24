package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type inMemoryTaskRepo struct {
	mu          sync.RWMutex
	tasks       map[uuid.UUID]*models.Task
	results     map[uuid.UUID]*models.TaskResult
	gateGets    int
	getArrived  chan struct{}
	releaseGets chan struct{}
}

func newInMemoryTaskRepo() *inMemoryTaskRepo {
	return &inMemoryTaskRepo{
		tasks:   make(map[uuid.UUID]*models.Task),
		results: make(map[uuid.UUID]*models.TaskResult),
	}
}

func (r *inMemoryTaskRepo) Create(ctx context.Context, task *models.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = cloneTask(task)
	return nil
}

func (r *inMemoryTaskRepo) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	r.mu.RLock()
	task, ok := r.tasks[id]
	if !ok {
		r.mu.RUnlock()
		return nil, ErrTaskNotFound
	}
	cloned := cloneTask(task)
	r.mu.RUnlock()

	var getArrived chan struct{}
	var releaseGets chan struct{}
	r.mu.Lock()
	if r.gateGets > 0 {
		r.gateGets--
		getArrived = r.getArrived
		releaseGets = r.releaseGets
	}
	r.mu.Unlock()

	if getArrived != nil {
		getArrived <- struct{}{}
		<-releaseGets
	}

	return cloned, nil
}

func (r *inMemoryTaskRepo) Update(ctx context.Context, task *models.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = cloneTask(task)
	return nil
}

func (r *inMemoryTaskRepo) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tasks := make([]*models.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, cloneTask(task))
	}
	return tasks, nil
}

func (r *inMemoryTaskRepo) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tasks := make([]*models.Task, 0)
	for _, task := range r.tasks {
		if task.Status == status {
			tasks = append(tasks, cloneTask(task))
		}
	}
	return tasks, nil
}

func (r *inMemoryTaskRepo) CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, task := range r.tasks {
		if task.Status == status {
			count++
		}
	}
	return count, nil
}

func (r *inMemoryTaskRepo) GetAll(ctx context.Context) ([]models.Task, error) {
	return nil, nil
}

func (r *inMemoryTaskRepo) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *result
	r.results[result.TaskID] = &cloned
	return nil
}

func (r *inMemoryTaskRepo) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.results[taskID]
	if !ok {
		return nil, nil
	}
	cloned := *result
	return &cloned, nil
}

func (r *inMemoryTaskRepo) GetTaskResults(ctx context.Context, taskIDs []uuid.UUID) (map[uuid.UUID]*models.TaskResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make(map[uuid.UUID]*models.TaskResult, len(taskIDs))
	for _, taskID := range taskIDs {
		result, ok := r.results[taskID]
		if !ok {
			continue
		}
		cloned := *result
		results[taskID] = &cloned
	}
	return results, nil
}

func (r *inMemoryTaskRepo) GetTasksByRunner(ctx context.Context, runnerID string, limit int) ([]*models.Task, error) {
	return nil, nil
}

type inMemoryRunnerRepo struct {
	mu      sync.RWMutex
	runners map[string]*models.Runner
}

func newInMemoryRunnerRepo() *inMemoryRunnerRepo {
	return &inMemoryRunnerRepo{
		runners: make(map[string]*models.Runner),
	}
}

func (r *inMemoryRunnerRepo) Create(ctx context.Context, runner *models.Runner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return nil
}

func (r *inMemoryRunnerRepo) Get(ctx context.Context, deviceID string) (*models.Runner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[deviceID]
	if !ok {
		return nil, ErrRunnerNotFound
	}
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) CreateOrUpdate(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) Update(ctx context.Context, runner *models.Runner) (*models.Runner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[runner.DeviceID] = cloneRunner(runner)
	return cloneRunner(runner), nil
}

func (r *inMemoryRunnerRepo) ListByStatus(ctx context.Context, status models.RunnerStatus) ([]*models.Runner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runners := make([]*models.Runner, 0)
	for _, runner := range r.runners {
		if runner.Status == status {
			runners = append(runners, cloneRunner(runner))
		}
	}
	return runners, nil
}

func (r *inMemoryRunnerRepo) ListAll(ctx context.Context) ([]*models.Runner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runners := make([]*models.Runner, 0, len(r.runners))
	for _, runner := range r.runners {
		runners = append(runners, cloneRunner(runner))
	}
	return runners, nil
}

func (r *inMemoryRunnerRepo) ListRecent(ctx context.Context, limit int) ([]*models.Runner, error) {
	runners, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(runners) > limit {
		return runners[:limit], nil
	}
	return runners, nil
}

func (r *inMemoryRunnerRepo) CountByStatus(ctx context.Context) (map[models.RunnerStatus]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[models.RunnerStatus]int64)
	for _, runner := range r.runners {
		status := runner.Status
		if status == models.RunnerStatusOnline && runner.TaskID != nil {
			status = models.RunnerStatusBusy
		}
		counts[status]++
	}
	return counts, nil
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

func newTestDockerTask(t *testing.T, title string) *models.Task {
	t.Helper()

	task := models.NewTask()
	task.Title = title
	task.Description = title
	task.Type = models.TaskTypeDocker
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

	return task
}

func newTestCommandTask(t *testing.T, title string) *models.Task {
	t.Helper()

	task := models.NewTask()
	task.Title = title
	task.Description = title
	task.Type = models.TaskTypeCommand
	task.Status = models.TaskStatusPending
	task.Nonce = fmt.Sprintf("%d", time.Now().UnixNano())

	config, err := json.Marshal(map[string]interface{}{
		"command":         "/bin/echo " + title,
		"timeout_seconds": 5,
	})
	if err != nil {
		t.Fatalf("failed to marshal command config: %v", err)
	}
	task.Config = config

	return task
}

func executeCommandTaskForStress(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	var config struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout_seconds"`
	}

	if err := json.Unmarshal(task.Config, &config); err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = 5
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
	defer cancel()

	parts := strings.Fields(config.Command)
	cmd := exec.CommandContext(cmdCtx, parts[0], parts[1:]...)
	startedAt := time.Now()
	output, err := cmd.CombinedOutput()

	result := models.NewTaskResult()
	result.TaskID = task.ID
	result.Output = string(output)
	result.ExecutionTime = time.Since(startedAt).Milliseconds()
	if result.ExecutionTime <= 0 {
		result.ExecutionTime = 1
	}

	if err != nil {
		result.Error = err.Error()
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
		} else {
			result.ExitCode = 1
		}
		return result, nil
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	return result, nil
}

func TestAssignTaskToRunnerAllowsOnlyOneConcurrentWinnerPerTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := newTestDockerTask(t, "shared-task")
	taskRepo.tasks[task.ID] = cloneTask(task)
	taskRepo.gateGets = 2
	taskRepo.getArrived = make(chan struct{}, 2)
	taskRepo.releaseGets = make(chan struct{})

	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		Webhook:  server.URL,
	}
	runnerRepo.runners["runner-2"] = &models.Runner{
		DeviceID: "runner-2",
		Status:   models.RunnerStatusOnline,
		Webhook:  server.URL,
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- taskService.assignTaskToRunner(context.Background(), task, &models.Runner{
			DeviceID: "runner-1",
			Status:   models.RunnerStatusOnline,
			Webhook:  server.URL,
		})
	}()
	go func() {
		defer wg.Done()
		errs <- taskService.assignTaskToRunner(context.Background(), task, &models.Runner{
			DeviceID: "runner-2",
			Status:   models.RunnerStatusOnline,
			Webhook:  server.URL,
		})
	}()

	deadline := time.After(200 * time.Millisecond)
	arrivals := 0
	for arrivals < 2 {
		select {
		case <-taskRepo.getArrived:
			arrivals++
		case <-deadline:
			arrivals = 2
		}
	}
	close(taskRepo.releaseGets)

	wg.Wait()
	close(errs)

	successes := 0
	conflicts := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrTaskUnavailable), errors.Is(err, ErrRunnerUnavailable):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successes != 1 {
		t.Fatalf("success count = %d, want 1", successes)
	}
	if conflicts != 1 {
		t.Fatalf("conflict count = %d, want 1", conflicts)
	}

	storedTask, err := taskRepo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("failed to get stored task: %v", err)
	}
	if storedTask.RunnerID == "" {
		t.Fatal("expected task to remain assigned to one runner")
	}

	winningRunner, err := runnerRepo.Get(context.Background(), storedTask.RunnerID)
	if err != nil {
		t.Fatalf("failed to get winning runner: %v", err)
	}
	if winningRunner.TaskID == nil || *winningRunner.TaskID != task.ID {
		t.Fatalf("winning runner task = %v, want %s", winningRunner.TaskID, task.ID.String())
	}

	losingRunnerID := "runner-1"
	if storedTask.RunnerID == losingRunnerID {
		losingRunnerID = "runner-2"
	}
	losingRunner, err := runnerRepo.Get(context.Background(), losingRunnerID)
	if err != nil {
		t.Fatalf("failed to get losing runner: %v", err)
	}
	if losingRunner.TaskID != nil {
		t.Fatalf("losing runner should be idle, got task %s", losingRunner.TaskID.String())
	}
}

func TestMultipleRunnersCanProcessBatchOfTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusOnline,
		Webhook:  server.URL,
	}
	runnerRepo.runners["runner-2"] = &models.Runner{
		DeviceID: "runner-2",
		Status:   models.RunnerStatusOnline,
		Webhook:  server.URL,
	}

	tasks := []*models.Task{
		newTestDockerTask(t, "task-1"),
		newTestDockerTask(t, "task-2"),
		newTestDockerTask(t, "task-3"),
	}
	for _, task := range tasks {
		taskRepo.tasks[task.ID] = cloneTask(task)
	}

	if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-1"); err != nil {
		t.Fatalf("assign pending tasks to runner-1: %v", err)
	}
	if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-2"); err != nil {
		t.Fatalf("assign pending tasks to runner-2: %v", err)
	}

	assignedTaskIDs := make(map[uuid.UUID]string)
	for _, runnerID := range []string{"runner-1", "runner-2"} {
		runner, err := runnerRepo.Get(context.Background(), runnerID)
		if err != nil {
			t.Fatalf("failed to get runner %s: %v", runnerID, err)
		}
		if runner.TaskID == nil {
			t.Fatalf("expected runner %s to have an assigned task", runnerID)
		}
		assignedTaskIDs[*runner.TaskID] = runnerID

		if err := taskService.StartTask(context.Background(), runner.TaskID.String()); err != nil {
			t.Fatalf("start task %s: %v", runner.TaskID.String(), err)
		}
	}

	if len(assignedTaskIDs) != 2 {
		t.Fatalf("assigned task count = %d, want 2", len(assignedTaskIDs))
	}

	for taskID, runnerID := range assignedTaskIDs {
		result := models.NewTaskResult()
		result.TaskID = taskID
		result.DeviceID = runnerID
		result.SolverDeviceID = runnerID

		if err := taskService.SaveTaskResult(context.Background(), result); err != nil {
			t.Fatalf("save task result for %s: %v", taskID.String(), err)
		}
	}

	if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-1"); err != nil {
		t.Fatalf("reassign pending tasks to runner-1: %v", err)
	}
	if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-2"); err != nil {
		t.Fatalf("reassign pending tasks to runner-2: %v", err)
	}

	var finalTask *models.Task
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pendingTasks, err := taskRepo.ListByStatus(context.Background(), models.TaskStatusPending)
		if err != nil {
			t.Fatalf("list pending tasks: %v", err)
		}
		if len(pendingTasks) == 1 && pendingTasks[0].RunnerID != "" {
			finalTask = pendingTasks[0]
			break
		}

		if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-1"); err != nil {
			t.Fatalf("retry reassign pending tasks to runner-1: %v", err)
		}
		if err := taskService.checkAndAssignPendingTasksToRunner(context.Background(), "runner-2"); err != nil {
			t.Fatalf("retry reassign pending tasks to runner-2: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
	}

	if finalTask == nil {
		t.Fatal("expected remaining task to be assigned after runners became available")
	}

	if err := taskService.StartTask(context.Background(), finalTask.ID.String()); err != nil {
		t.Fatalf("start final task: %v", err)
	}

	finalResult := models.NewTaskResult()
	finalResult.TaskID = finalTask.ID
	finalResult.DeviceID = finalTask.RunnerID
	finalResult.SolverDeviceID = finalTask.RunnerID
	if err := taskService.SaveTaskResult(context.Background(), finalResult); err != nil {
		t.Fatalf("save final task result: %v", err)
	}

	completedTasks, err := taskRepo.ListByStatus(context.Background(), models.TaskStatusCompleted)
	if err != nil {
		t.Fatalf("list completed tasks: %v", err)
	}
	if len(completedTasks) != 3 {
		t.Fatalf("completed task count = %d, want 3", len(completedTasks))
	}
}

func TestMultiRunnerStressExecutesCommandTasks(t *testing.T) {
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	runnerIDs := []string{"runner-1", "runner-2", "runner-3", "runner-4"}
	for _, runnerID := range runnerIDs {
		runnerRepo.runners[runnerID] = &models.Runner{
			DeviceID: runnerID,
			Status:   models.RunnerStatusOnline,
			Webhook:  webhookServer.URL,
		}
	}

	const taskCount = 40
	for i := 0; i < taskCount; i++ {
		task := newTestCommandTask(t, fmt.Sprintf("stress-task-%02d", i))
		taskRepo.tasks[task.ID] = cloneTask(task)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, len(runnerIDs))
	var wg sync.WaitGroup

	for _, runnerID := range runnerIDs {
		runnerID := runnerID
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				if countTerminalTasks(taskRepo) == taskCount {
					return
				}

				task, err := pickVisiblePendingTask(context.Background(), taskService, runnerID)
				if err != nil {
					errCh <- fmt.Errorf("%s: list pending tasks: %w", runnerID, err)
					return
				}
				if task == nil {
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Millisecond):
						continue
					}
				}

				if err := taskService.AssignTaskToRunner(context.Background(), task.ID.String(), runnerID); err != nil {
					if errors.Is(err, ErrTaskUnavailable) || errors.Is(err, ErrRunnerUnavailable) {
						continue
					}
					errCh <- fmt.Errorf("%s: assign task %s: %w", runnerID, task.ID.String(), err)
					return
				}

				if err := taskService.StartTask(context.Background(), task.ID.String()); err != nil {
					if strings.Contains(err.Error(), "task is not in pending status") {
						continue
					}
					errCh <- fmt.Errorf("%s: start task %s: %w", runnerID, task.ID.String(), err)
					return
				}

				currentTask, err := taskService.GetTask(context.Background(), task.ID.String())
				if err != nil {
					errCh <- fmt.Errorf("%s: reload task %s: %w", runnerID, task.ID.String(), err)
					return
				}

				result, err := executeCommandTaskForStress(ctx, currentTask)
				if err != nil {
					errCh <- fmt.Errorf("%s: execute task %s: %w", runnerID, currentTask.ID.String(), err)
					return
				}
				result.DeviceID = runnerID
				result.SolverDeviceID = runnerID

				if err := taskService.SaveTaskResult(context.Background(), result); err != nil {
					errCh <- fmt.Errorf("%s: save result for task %s: %w", runnerID, currentTask.ID.String(), err)
					return
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case err := <-errCh:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatalf("stress run timed out; pending=%d running=%d completed=%d failed=%d not_verified=%d",
			countStatus(taskRepo, models.TaskStatusPending),
			countStatus(taskRepo, models.TaskStatusRunning),
			countStatus(taskRepo, models.TaskStatusCompleted),
			countStatus(taskRepo, models.TaskStatusFailed),
			countStatus(taskRepo, models.TaskStatusNotVerified),
		)
	}

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}

	if completed := countStatus(taskRepo, models.TaskStatusCompleted); completed != taskCount {
		t.Fatalf("completed task count = %d, want %d", completed, taskCount)
	}
	if failed := countStatus(taskRepo, models.TaskStatusFailed); failed != 0 {
		t.Fatalf("failed task count = %d, want 0", failed)
	}
	if running := countStatus(taskRepo, models.TaskStatusRunning); running != 0 {
		t.Fatalf("running task count = %d, want 0", running)
	}
	if pending := countStatus(taskRepo, models.TaskStatusPending); pending != 0 {
		t.Fatalf("pending task count = %d, want 0", pending)
	}

	allTasks, err := taskRepo.List(context.Background(), taskCount, 0)
	if err != nil {
		t.Fatalf("list all tasks: %v", err)
	}
	for _, task := range allTasks {
		taskID := task.ID
		result, err := taskRepo.GetTaskResult(context.Background(), taskID)
		if err != nil {
			t.Fatalf("get result for %s: %v", taskID.String(), err)
		}
		if result == nil {
			t.Fatalf("missing result for task %s", taskID.String())
		}
		if strings.TrimSpace(result.Output) != task.Title {
			t.Fatalf("result output for task %s = %q, want %q", taskID.String(), strings.TrimSpace(result.Output), task.Title)
		}
	}

	for _, runnerID := range runnerIDs {
		runner, err := runnerRepo.Get(context.Background(), runnerID)
		if err != nil {
			t.Fatalf("get runner %s: %v", runnerID, err)
		}
		if runner.TaskID != nil {
			t.Fatalf("runner %s still has task %s", runnerID, runner.TaskID.String())
		}
		if runner.Status != models.RunnerStatusOnline {
			t.Fatalf("runner %s status = %s, want %s", runnerID, runner.Status, models.RunnerStatusOnline)
		}
	}
}

func pickVisiblePendingTask(ctx context.Context, taskService *TaskService, runnerID string) (*models.Task, error) {
	tasks, err := taskService.ListAvailableTasks(ctx)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.RunnerID == "" || task.RunnerID == runnerID {
			return task, nil
		}
	}

	return nil, nil
}

func countTerminalTasks(taskRepo *inMemoryTaskRepo) int {
	return countStatus(taskRepo, models.TaskStatusCompleted) +
		countStatus(taskRepo, models.TaskStatusFailed) +
		countStatus(taskRepo, models.TaskStatusNotVerified)
}

func countStatus(taskRepo *inMemoryTaskRepo, status models.TaskStatus) int {
	tasks, err := taskRepo.ListByStatus(context.Background(), status)
	if err != nil {
		return 0
	}
	return len(tasks)
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

func TestSaveTaskResultMarksFailedWhenExitCodeNonZero(t *testing.T) {
	taskRepo := newInMemoryTaskRepo()
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	taskService := NewTaskService(taskRepo, nil, runnerService)

	task := models.NewTask()
	task.Title = "failed"
	task.Description = "non-zero exit"
	task.Type = models.TaskTypeCommand
	task.Status = models.TaskStatusRunning
	task.RunnerID = "runner-1"
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
	result.ExitCode = 2
	result.Error = "command failed"

	if err := taskService.SaveTaskResult(context.Background(), result); err != nil {
		t.Fatalf("SaveTaskResult() error = %v", err)
	}

	storedTask, err := taskRepo.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if storedTask.Status != models.TaskStatusFailed {
		t.Fatalf("task status = %q, want %q", storedTask.Status, models.TaskStatusFailed)
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
