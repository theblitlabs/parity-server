package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/models"
	"github.com/theblitlabs/parity-server/internal/services"
)

type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RunnerID  string    `json:"runner_id"`
	DeviceID  string    `json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL      string `json:"url"`
	RunnerID string `json:"runner_id"`
	DeviceID string `json:"device_id"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type CreateTaskRequest struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Type        models.TaskType           `json:"type"`
	Config      json.RawMessage           `json:"config"`
	Environment *models.EnvironmentConfig `json:"environment,omitempty"`
	Reward      float64                   `json:"reward"`
	CreatorID   string                    `json:"creator_id"`
}

type TaskService interface {
	CreateTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	ListAvailableTasks(ctx context.Context) ([]*models.Task, error)
	AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error
	GetTaskReward(ctx context.Context, taskID string) (float64, error)
	GetTasks(ctx context.Context) ([]models.Task, error)
	StartTask(ctx context.Context, id string) error
	CompleteTask(ctx context.Context, id string) error
	SaveTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error)
}

type TaskHandler struct {
	service      TaskService
	stakeWallet  *walletsdk.StakeWallet
	taskUpdateCh chan struct{}
	webhooks     map[string]WebhookRegistration
	webhookMutex sync.RWMutex
	stopCh       chan struct{}
}

func NewTaskHandler(service TaskService, webhookService *services.WebhookService) *TaskHandler {
	return &TaskHandler{
		service:        service,
		webhookService: webhookService,
	}
}

func (h *TaskHandler) SetStakeWallet(wallet *walletsdk.StakeWallet) {
	h.stakeWallet = wallet
}

func (h *TaskHandler) SetStopChannel(stopCh chan struct{}) {
	h.stopCh = stopCh
	h.webhookService.SetStopChannel(stopCh)
}

func (h *TaskHandler) NotifyTaskUpdate() {
	h.webhookService.NotifyTaskUpdate()
}

func (h *TaskHandler) RegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var req services.RegisterWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	webhookID, err := h.webhookService.RegisterWebhook(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id": webhookID,
	}); err != nil {
		log := gologger.Get()
		log.Error().Err(err).Str("webhook_id", webhookID).Msg("Failed to encode webhook registration response")
	}
}

func (h *TaskHandler) UnregisterWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := mux.Vars(r)["id"]
	if webhookID == "" {
		http.Error(w, "Webhook ID is required", http.StatusBadRequest)
		return
	}

	if err := h.webhookService.UnregisterWebhook(webhookID); err != nil {
		if err.Error() == "webhook not found" {
			http.Error(w, "Webhook not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) GetTaskResult(w http.ResponseWriter, r *http.Request) {
	log := gologger.Get()
	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetTaskResult(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "task result not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to encode task result response")
	}
}

func generateNonce() string {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		nonceBytes = []byte(fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()))
	}
	return hex.EncodeToString(nonceBytes)
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log := gologger.Get()
	log.Info().
		Str("request", fmt.Sprintf("%+v", req)).
		Msg("Creating task")

	if req.Title == "" || req.Description == "" {
		http.Error(w, "Title and description are required", http.StatusBadRequest)
		return
	}

	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	creatorAddress := r.Header.Get("X-Creator-Address")
	if creatorAddress == "" {
		http.Error(w, "Creator address is required", http.StatusBadRequest)
		return
	}

	if req.Type != models.TaskTypeDocker && req.Type != models.TaskTypeCommand {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == models.TaskTypeDocker {
		if len(req.Config) == 0 {
			http.Error(w, "Command is required for Docker tasks", http.StatusBadRequest)
			return
		}
		if req.Environment == nil || req.Environment.Type != "docker" {
			http.Error(w, "Docker environment configuration is required", http.StatusBadRequest)
			return
		}
	}

	nonce := generateNonce()

	log.Debug().
		Str("nonce", nonce).
		Msg("Generated nonce")

	task := models.NewTask()
	task.Title = req.Title
	task.Description = req.Description
	task.Type = req.Type
	task.Config = req.Config
	task.Environment = req.Environment
	task.CreatorDeviceID = deviceID
	task.CreatorAddress = creatorAddress
	task.Nonce = nonce

	log.Debug().
		Str("task_id", task.ID.String()).
		Str("creator_device_id", task.CreatorDeviceID).
		Str("creator_address", task.CreatorAddress).
		Str("nonce", task.Nonce).
		Msg("Creating task")

	if err := h.checkStakeBalance(task); err != nil {
		log.Error().Err(err).
			Str("device_id", deviceID).
			Msg("Insufficient stake balance for task reward")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.CreateTask(r.Context(), task); err != nil {
		log.Error().Err(err).Msg("Failed to create task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.NotifyTaskUpdate()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		log.Error().Err(err).Msg("Failed to encode task response")
	}
}

func (h *TaskHandler) StartTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := gologger.Get()

	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	runnerID := r.Header.Get("X-Runner-ID")
	if runnerID == "" {
		http.Error(w, "X-Runner-ID header is required", http.StatusBadRequest)
		return
	}

	if err := h.service.AssignTaskToRunner(ctx, taskID, runnerID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to assign task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.service.StartTask(ctx, taskID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to start task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.NotifyTaskUpdate()

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) SaveTaskResult(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("task_handler")
	vars := mux.Vars(r)
	taskID := vars["id"]
	deviceID := r.Header.Get("X-Device-ID")

	if deviceID == "" {
		log.Debug().Str("task", taskID).Msg("Missing device ID")
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	// First get the task to ensure it exists and get its data
	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		log.Error().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Task fetch failed")
		http.Error(w, "Task fetch failed", http.StatusInternalServerError)
		return
	}

	if task == nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Task not found")
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Debug().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid result payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid task ID")
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Populate result with data from the existing task
	result.TaskID = taskUUID
	result.DeviceID = deviceID
	result.CreatorDeviceID = task.CreatorDeviceID
	result.SolverDeviceID = deviceID

	// If creator address is not set in task, use creator device ID as temporary address
	if task.CreatorAddress == "" {
		result.CreatorAddress = task.CreatorDeviceID
		log.Debug().
			Str("task", taskID).
			Str("creator_device", task.CreatorDeviceID).
			Msg("Using creator device ID as temporary creator address")
	} else {
		result.CreatorAddress = task.CreatorAddress
	}

	result.RunnerAddress = deviceID
	result.CreatedAt = time.Now()

	// Calculate device ID hash
	hash := sha256.Sum256([]byte(deviceID))
	result.DeviceIDHash = hex.EncodeToString(hash[:])
	result.Clean()

	log.Info().
		Str("task", taskID).
		Str("creator_address", result.CreatorAddress).
		Str("creator_device", result.CreatorDeviceID).
		Str("solver_device", result.SolverDeviceID).
		Str("runner_address", result.RunnerAddress).
		Msg("Saving task result")

	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		if strings.Contains(err.Error(), "invalid task result:") {
			log.Info().Err(err).
				Str("task", taskID).
				Str("error", err.Error()).
				Msg("Invalid result")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Error().Err(err).Str("task", taskID).Msg("Failed to save task result")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("task", taskID).
		Str("creator_device", task.CreatorDeviceID).
		Str("solver_device", deviceID).
		Msg("Task result saved")

	h.NotifyTaskUpdate()

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) checkStakeBalance(task *models.Task) error {
	if h.stakeWallet == nil {
		return fmt.Errorf("stake wallet not initialized")
	}

	stakeInfo, err := h.stakeWallet.GetStakeInfo(task.CreatorDeviceID)
	if err != nil || !stakeInfo.Exists {
		return fmt.Errorf("creator device not registered - please stake first")
	}

	if stakeInfo.Amount.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("no stake found - please stake some PRTY first")
	}

	return nil
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	log := gologger.Get()
	tasks, err := h.service.GetTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Error().Err(err).Msg("Failed to encode tasks response")
	}
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	log := gologger.Get()
	taskID := mux.Vars(r)["id"]
	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		log.Error().Err(err).Msg("Failed to encode task response")
	}
}

func (h *TaskHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	var req struct {
		RunnerID string `json:"runner_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.RunnerID == "" {
		http.Error(w, "Runner ID is required", http.StatusBadRequest)
		return
	}
	if err := h.service.AssignTaskToRunner(r.Context(), taskID, req.RunnerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.NotifyTaskUpdate()
	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) GetTaskReward(w http.ResponseWriter, r *http.Request) {
	log := gologger.Get()
	taskID := mux.Vars(r)["id"]
	reward, err := h.service.GetTaskReward(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reward); err != nil {
		log.Error().Err(err).Msg("Failed to encode reward response")
	}
}

func (h *TaskHandler) ListAvailableTasks(w http.ResponseWriter, r *http.Request) {
	log := gologger.Get()
	tasks, err := h.service.ListAvailableTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Error().Err(err).Msg("Failed to encode available tasks response")
	}
}

func (h *TaskHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	if err := h.service.CompleteTask(r.Context(), taskID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.NotifyTaskUpdate()
	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) CleanupResources() {
	h.webhookService.CleanupResources()
}
