package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type QueuedTask struct {
	PromptID   uuid.UUID
	ModelName  string
	QueuedAt   time.Time
	RetryCount int
	MaxRetries int
}

type TaskQueue struct {
	promptRepo    ports.PromptRepository
	runnerRepo    ports.RunnerRepository
	runnerService *RunnerService
	queue         []QueuedTask
	mu            sync.RWMutex
	stopCh        chan struct{}
	running       bool
}

func NewTaskQueue(promptRepo ports.PromptRepository, runnerRepo ports.RunnerRepository, runnerService *RunnerService) *TaskQueue {
	return &TaskQueue{
		promptRepo:    promptRepo,
		runnerRepo:    runnerRepo,
		runnerService: runnerService,
		queue:         make([]QueuedTask, 0),
		stopCh:        make(chan struct{}),
	}
}

func (tq *TaskQueue) Start(ctx context.Context) {
	tq.mu.Lock()
	if tq.running {
		tq.mu.Unlock()
		return
	}
	tq.running = true
	tq.mu.Unlock()

	log := gologger.WithComponent("task_queue")
	log.Info().Msg("Starting task queue processor")

	if err := tq.restoreQueuedPrompts(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to restore queued prompts from storage")
	}

	tq.processQueue(ctx)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Task queue processor stopped due to context cancellation")
			return
		case <-tq.stopCh:
			log.Info().Msg("Task queue processor stopped")
			return
		case <-ticker.C:
			tq.processQueue(ctx)
		}
	}
}

func (tq *TaskQueue) Stop() {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if !tq.running {
		return
	}

	close(tq.stopCh)
	tq.running = false
}

func (tq *TaskQueue) QueueTask(promptID uuid.UUID, modelName string) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task := QueuedTask{
		PromptID:   promptID,
		ModelName:  modelName,
		QueuedAt:   time.Now(),
		RetryCount: 0,
		MaxRetries: 5,
	}

	if !tq.enqueueLocked(task) {
		return
	}

	log := gologger.WithComponent("task_queue")
	log.Info().
		Str("prompt_id", promptID.String()).
		Str("model_name", modelName).
		Int("queue_size", len(tq.queue)).
		Msg("Task queued for processing")
}

func (tq *TaskQueue) enqueueLocked(task QueuedTask) bool {
	for i := range tq.queue {
		if tq.queue[i].PromptID != task.PromptID {
			continue
		}

		if tq.queue[i].ModelName == "" {
			tq.queue[i].ModelName = task.ModelName
		}
		if tq.queue[i].QueuedAt.IsZero() || (!task.QueuedAt.IsZero() && task.QueuedAt.Before(tq.queue[i].QueuedAt)) {
			tq.queue[i].QueuedAt = task.QueuedAt
		}
		if task.MaxRetries > tq.queue[i].MaxRetries {
			tq.queue[i].MaxRetries = task.MaxRetries
		}
		return false
	}

	tq.queue = append(tq.queue, task)
	return true
}

func (tq *TaskQueue) restoreQueuedPrompts(ctx context.Context) error {
	log := gologger.WithComponent("task_queue")

	prompts, err := tq.promptRepo.GetQueuedPrompts(ctx)
	if err != nil {
		return err
	}

	if len(prompts) == 0 {
		return nil
	}

	tq.mu.Lock()
	restored := 0
	for _, prompt := range prompts {
		if prompt == nil {
			continue
		}

		if tq.enqueueLocked(QueuedTask{
			PromptID:   prompt.ID,
			ModelName:  prompt.ModelName,
			QueuedAt:   prompt.CreatedAt,
			MaxRetries: 5,
		}) {
			restored++
		}
	}
	queueSize := len(tq.queue)
	tq.mu.Unlock()

	log.Info().
		Int("restored_count", restored).
		Int("queue_size", queueSize).
		Msg("Restored queued prompts from storage")

	return nil
}

func (tq *TaskQueue) processQueue(ctx context.Context) {
	tq.mu.RLock()
	if len(tq.queue) == 0 {
		tq.mu.RUnlock()
		return
	}

	queueCopy := make([]QueuedTask, len(tq.queue))
	copy(queueCopy, tq.queue)
	tq.mu.RUnlock()

	log := gologger.WithComponent("task_queue")

	processedPromptIDs := make(map[uuid.UUID]struct{})
	updatedTasks := make(map[uuid.UUID]QueuedTask)

	for _, task := range queueCopy {
		updatedTask, processed := tq.processTask(ctx, task)
		if processed {
			processedPromptIDs[task.PromptID] = struct{}{}
			continue
		}
		updatedTasks[task.PromptID] = updatedTask
	}

	if len(processedPromptIDs) == 0 && len(updatedTasks) == 0 {
		return
	}

	tq.mu.Lock()
	filteredQueue := make([]QueuedTask, 0, len(tq.queue))
	for _, task := range tq.queue {
		if _, remove := processedPromptIDs[task.PromptID]; remove {
			continue
		}
		if updatedTask, ok := updatedTasks[task.PromptID]; ok {
			task = updatedTask
		}
		filteredQueue = append(filteredQueue, task)
	}
	tq.queue = filteredQueue
	remainingQueueSize := len(tq.queue)
	tq.mu.Unlock()

	if len(processedPromptIDs) > 0 {
		log.Info().
			Int("processed_count", len(processedPromptIDs)).
			Int("remaining_queue_size", remainingQueueSize).
			Msg("Processed queued tasks")
	}
}

func (tq *TaskQueue) processTask(ctx context.Context, task QueuedTask) (QueuedTask, bool) {
	log := gologger.WithComponent("task_queue")

	promptReq, err := tq.promptRepo.GetByID(ctx, task.PromptID)
	if err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", task.PromptID.String()).
			Msg("Failed to get prompt request from database")
		return task, true
	}

	if promptReq.Status != models.PromptStatusQueued {
		log.Info().
			Str("prompt_id", task.PromptID.String()).
			Str("status", string(promptReq.Status)).
			Msg("Prompt is no longer queued, removing from queue")
		return task, true
	}

	runnerID, err := tq.runnerService.GetAvailableRunnerForModel(ctx, task.ModelName)
	if err != nil {
		task.RetryCount++
		if task.RetryCount >= task.MaxRetries {
			log.Warn().
				Str("prompt_id", task.PromptID.String()).
				Str("model_name", task.ModelName).
				Int("retry_count", task.RetryCount).
				Msg("Max retries reached, marking prompt as failed")

			now := time.Now()
			promptReq.Status = models.PromptStatusFailed
			promptReq.CompletedAt = &now
			if updateErr := tq.promptRepo.Update(ctx, promptReq); updateErr != nil {
				log.Error().
					Err(updateErr).
					Str("prompt_id", task.PromptID.String()).
					Msg("Failed to update prompt status to failed")
			}
			return task, true
		}

		log.Debug().
			Str("prompt_id", task.PromptID.String()).
			Str("model_name", task.ModelName).
			Int("retry_count", task.RetryCount).
			Msg("No runner available yet, will retry later")
		return task, false
	}

	promptReq.RunnerID = runnerID
	promptReq.Status = models.PromptStatusProcessing

	if err := tq.promptRepo.Update(ctx, promptReq); err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", task.PromptID.String()).
			Msg("Failed to update prompt status to processing")
		return task, false
	}

	go func() {
		bgCtx := context.Background()
		if err := tq.runnerService.ForwardPromptToRunner(bgCtx, runnerID, promptReq); err != nil {
			log.Error().
				Err(err).
				Str("runner_id", runnerID).
				Str("prompt_id", task.PromptID.String()).
				Msg("Failed to forward prompt to runner - marking prompt as failed")

			now := time.Now()
			promptReq.Status = models.PromptStatusFailed
			promptReq.CompletedAt = &now
			if updateErr := tq.promptRepo.Update(bgCtx, promptReq); updateErr != nil {
				log.Error().
					Err(updateErr).
					Str("prompt_id", promptReq.ID.String()).
					Msg("Failed to update prompt status to failed")
			}

			// Free up the runner by clearing its TaskID if it was assigned
			if promptReq.RunnerID != "" {
				if runner, err := tq.runnerRepo.GetRunnerByDeviceID(bgCtx, promptReq.RunnerID); err == nil {
					runner.TaskID = nil
					if _, updateErr := tq.runnerService.UpdateRunner(bgCtx, runner); updateErr != nil {
						log.Error().
							Err(updateErr).
							Str("runner_id", promptReq.RunnerID).
							Msg("Failed to clear runner TaskID after failure")
					} else {
						log.Info().
							Str("runner_id", promptReq.RunnerID).
							Msg("Runner freed after prompt failure in queue processing")
					}
				}
			}
		}
	}()

	log.Info().
		Str("prompt_id", task.PromptID.String()).
		Str("model_name", task.ModelName).
		Str("runner_id", runnerID).
		Msg("Queued task processed successfully")

	return task, true
}

func (tq *TaskQueue) GetQueueSize() int {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return len(tq.queue)
}

func (tq *TaskQueue) GetQueuedTasks() []QueuedTask {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	tasks := make([]QueuedTask, len(tq.queue))
	copy(tasks, tq.queue)
	return tasks
}
