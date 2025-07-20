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

	tq.queue = append(tq.queue, task)

	log := gologger.WithComponent("task_queue")
	log.Info().
		Str("prompt_id", promptID.String()).
		Str("model_name", modelName).
		Int("queue_size", len(tq.queue)).
		Msg("Task queued for processing")
}

func (tq *TaskQueue) processQueue(ctx context.Context) {
	tq.mu.Lock()
	if len(tq.queue) == 0 {
		tq.mu.Unlock()
		return
	}

	queueCopy := make([]QueuedTask, len(tq.queue))
	copy(queueCopy, tq.queue)
	tq.mu.Unlock()

	log := gologger.WithComponent("task_queue")

	var processedTasks []int

	for i, task := range queueCopy {
		processed := tq.processTask(ctx, task)
		if processed {
			processedTasks = append(processedTasks, i)
		}
	}

	if len(processedTasks) > 0 {
		tq.mu.Lock()
		// Remove processed tasks from queue (in reverse order to maintain indices)
		for i := len(processedTasks) - 1; i >= 0; i-- {
			idx := processedTasks[i]
			if idx < len(tq.queue) {
				tq.queue = append(tq.queue[:idx], tq.queue[idx+1:]...)
			}
		}
		log.Info().
			Int("processed_count", len(processedTasks)).
			Int("remaining_queue_size", len(tq.queue)).
			Msg("Processed queued tasks")
		tq.mu.Unlock()
	}
}

func (tq *TaskQueue) processTask(ctx context.Context, task QueuedTask) bool {
	log := gologger.WithComponent("task_queue")

	promptReq, err := tq.promptRepo.GetByID(ctx, task.PromptID)
	if err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", task.PromptID.String()).
			Msg("Failed to get prompt request from database")
		return true // Remove from queue as it's not recoverable
	}

	if promptReq.Status != models.PromptStatusQueued {
		log.Info().
			Str("prompt_id", task.PromptID.String()).
			Str("status", string(promptReq.Status)).
			Msg("Prompt is no longer queued, removing from queue")
		return true // Remove from queue as it's no longer queued
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
			return true // Remove from queue
		}

		log.Debug().
			Str("prompt_id", task.PromptID.String()).
			Str("model_name", task.ModelName).
			Int("retry_count", task.RetryCount).
			Msg("No runner available yet, will retry later")
		return false // Keep in queue for retry
	}

	promptReq.RunnerID = runnerID
	promptReq.Status = models.PromptStatusProcessing

	if err := tq.promptRepo.Update(ctx, promptReq); err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", task.PromptID.String()).
			Msg("Failed to update prompt status to processing")
		return false // Keep in queue for retry
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

	return true // Remove from queue
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
