package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

func TestTaskQueueRestoresQueuedPromptsFromStorage(t *testing.T) {
	promptRepo := newInMemoryPromptRepo()
	runnerRepo := newInMemoryRunnerRepo()
	queue := NewTaskQueue(promptRepo, runnerRepo, NewRunnerService(runnerRepo))

	prompt := models.NewPromptRequest("client-1", "hello", "model-a", "0xabc")
	prompt.Status = models.PromptStatusQueued
	promptRepo.prompts[prompt.ID] = clonePrompt(prompt)

	if err := queue.restoreQueuedPrompts(context.Background()); err != nil {
		t.Fatalf("restoreQueuedPrompts() error = %v", err)
	}

	queuedTasks := queue.GetQueuedTasks()
	if len(queuedTasks) != 1 {
		t.Fatalf("queued task count = %d, want 1", len(queuedTasks))
	}
	if queuedTasks[0].PromptID != prompt.ID {
		t.Fatalf("queued prompt ID = %s, want %s", queuedTasks[0].PromptID, prompt.ID)
	}
}

func TestTaskQueuePersistsRetryCountAndFailsAfterMaxRetries(t *testing.T) {
	promptRepo := newInMemoryPromptRepo()
	runnerRepo := newInMemoryRunnerRepo()
	queue := NewTaskQueue(promptRepo, runnerRepo, NewRunnerService(runnerRepo))

	prompt := models.NewPromptRequest("client-1", "hello", "model-a", "0xabc")
	prompt.Status = models.PromptStatusQueued
	promptRepo.prompts[prompt.ID] = clonePrompt(prompt)

	queue.QueueTask(prompt.ID, prompt.ModelName)

	for attempt := 1; attempt < 5; attempt++ {
		queue.processQueue(context.Background())

		queuedTasks := queue.GetQueuedTasks()
		if len(queuedTasks) != 1 {
			t.Fatalf("attempt %d: queued task count = %d, want 1", attempt, len(queuedTasks))
		}
		if queuedTasks[0].RetryCount != attempt {
			t.Fatalf("attempt %d: retry count = %d, want %d", attempt, queuedTasks[0].RetryCount, attempt)
		}
	}

	queue.processQueue(context.Background())

	if queue.GetQueueSize() != 0 {
		t.Fatalf("queue size = %d, want 0 after max retries", queue.GetQueueSize())
	}

	storedPrompt, err := promptRepo.GetByID(context.Background(), prompt.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if storedPrompt.Status != models.PromptStatusFailed {
		t.Fatalf("prompt status = %q, want %q", storedPrompt.Status, models.PromptStatusFailed)
	}
	if storedPrompt.CompletedAt == nil {
		t.Fatal("expected prompt to have completion timestamp after max retries")
	}
}

func TestTaskQueueDeduplicatesPromptEnqueue(t *testing.T) {
	promptRepo := newInMemoryPromptRepo()
	runnerRepo := newInMemoryRunnerRepo()
	queue := NewTaskQueue(promptRepo, runnerRepo, NewRunnerService(runnerRepo))

	promptID := uuid.New()
	queue.QueueTask(promptID, "model-a")
	queue.QueueTask(promptID, "model-a")

	if queue.GetQueueSize() != 1 {
		t.Fatalf("queue size = %d, want 1", queue.GetQueueSize())
	}
}
