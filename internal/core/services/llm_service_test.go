package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type inMemoryPromptRepo struct {
	prompts map[uuid.UUID]*models.PromptRequest
}

func newInMemoryPromptRepo() *inMemoryPromptRepo {
	return &inMemoryPromptRepo{
		prompts: make(map[uuid.UUID]*models.PromptRequest),
	}
}

func (r *inMemoryPromptRepo) Create(ctx context.Context, prompt *models.PromptRequest) error {
	r.prompts[prompt.ID] = clonePrompt(prompt)
	return nil
}

func (r *inMemoryPromptRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.PromptRequest, error) {
	prompt, ok := r.prompts[id]
	if !ok {
		return nil, errors.New("prompt not found")
	}
	return clonePrompt(prompt), nil
}

func (r *inMemoryPromptRepo) Update(ctx context.Context, prompt *models.PromptRequest) error {
	r.prompts[prompt.ID] = clonePrompt(prompt)
	return nil
}

func (r *inMemoryPromptRepo) ListByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.PromptRequest, error) {
	return nil, nil
}

func (r *inMemoryPromptRepo) GetPendingPrompts(ctx context.Context) ([]*models.PromptRequest, error) {
	return nil, nil
}

type inMemoryBillingRepo struct {
	createCalls int
	metrics     []*models.BillingMetric
}

func (r *inMemoryBillingRepo) Create(ctx context.Context, metric *models.BillingMetric) error {
	r.createCalls++
	r.metrics = append(r.metrics, metric)
	return nil
}

func (r *inMemoryBillingRepo) GetAggregatedMetrics(ctx context.Context, clientID string) (*models.BillingMetric, error) {
	return &models.BillingMetric{}, nil
}

func (r *inMemoryBillingRepo) GetMetricsByClientID(ctx context.Context, clientID string, limit, offset int) ([]*models.BillingMetric, error) {
	return nil, nil
}

func clonePrompt(prompt *models.PromptRequest) *models.PromptRequest {
	cloned := *prompt
	if prompt.CompletedAt != nil {
		completedAt := *prompt.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	return &cloned
}

func TestCompletePromptRejectsDifferentRunner(t *testing.T) {
	promptRepo := newInMemoryPromptRepo()
	billingRepo := &inMemoryBillingRepo{}
	runnerRepo := newInMemoryRunnerRepo()
	service := NewLLMService(promptRepo, billingRepo, runnerRepo, NewRunnerService(runnerRepo), nil)

	prompt := models.NewPromptRequest("client-1", "hello", "model-a", "0xabc")
	prompt.RunnerID = "runner-1"
	prompt.Status = models.PromptStatusProcessing
	promptRepo.prompts[prompt.ID] = clonePrompt(prompt)

	err := service.CompletePrompt(context.Background(), prompt.ID, "runner-2", "response", 10, 20, 30)
	if !errors.Is(err, ErrPromptRunnerMismatch) {
		t.Fatalf("CompletePrompt() error = %v, want %v", err, ErrPromptRunnerMismatch)
	}

	storedPrompt, getErr := promptRepo.GetByID(context.Background(), prompt.ID)
	if getErr != nil {
		t.Fatalf("GetByID() error = %v", getErr)
	}

	if storedPrompt.Status != models.PromptStatusProcessing {
		t.Fatalf("prompt status = %q, want %q", storedPrompt.Status, models.PromptStatusProcessing)
	}

	if billingRepo.createCalls != 0 {
		t.Fatalf("billing create calls = %d, want 0", billingRepo.createCalls)
	}
}

func TestCompletePromptIsIdempotentAfterSuccess(t *testing.T) {
	promptRepo := newInMemoryPromptRepo()
	billingRepo := &inMemoryBillingRepo{}
	runnerRepo := newInMemoryRunnerRepo()
	runnerService := NewRunnerService(runnerRepo)
	service := NewLLMService(promptRepo, billingRepo, runnerRepo, runnerService, nil)

	taskID := uuid.New()
	runnerRepo.runners["runner-1"] = &models.Runner{
		DeviceID: "runner-1",
		Status:   models.RunnerStatusBusy,
		TaskID:   &taskID,
	}

	prompt := models.NewPromptRequest("client-1", "hello", "model-a", "0xabc")
	prompt.RunnerID = "runner-1"
	prompt.Status = models.PromptStatusProcessing
	promptRepo.prompts[prompt.ID] = clonePrompt(prompt)

	if err := service.CompletePrompt(context.Background(), prompt.ID, "runner-1", "response", 10, 20, 30); err != nil {
		t.Fatalf("first CompletePrompt() error = %v", err)
	}

	if err := service.CompletePrompt(context.Background(), prompt.ID, "runner-1", "response", 10, 20, 30); err != nil {
		t.Fatalf("second CompletePrompt() error = %v", err)
	}

	if billingRepo.createCalls != 1 {
		t.Fatalf("billing create calls = %d, want 1", billingRepo.createCalls)
	}

	storedPrompt, getErr := promptRepo.GetByID(context.Background(), prompt.ID)
	if getErr != nil {
		t.Fatalf("GetByID() error = %v", getErr)
	}

	if storedPrompt.Status != models.PromptStatusCompleted {
		t.Fatalf("prompt status = %q, want %q", storedPrompt.Status, models.PromptStatusCompleted)
	}
}
