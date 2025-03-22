package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
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
	Image       string                    `json:"image"`
	Command     []string                  `json:"command"`
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
	service        TaskService
	webhookService *services.WebhookService
	s3Service      *services.S3Service
	stakeWallet    *walletsdk.StakeWallet
	webhooks       map[string]WebhookRegistration
	stopCh         chan struct{}
	runnerService  *services.RunnerService
}

func NewTaskHandler(service TaskService, webhookService *services.WebhookService, runnerService *services.RunnerService, s3Service *services.S3Service) *TaskHandler {
	return &TaskHandler{
		service:        service,
		webhookService: webhookService,
		s3Service:      s3Service,
		webhooks:       make(map[string]WebhookRegistration),
		runnerService:  runnerService,
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

	runner, err := h.runnerService.CreateOrUpdateRunner(r.Context(), &models.Runner{
		DeviceID:      req.DeviceID,
		Status:        models.RunnerStatusOnline,
		Webhook:       req.URL,
		WalletAddress: req.WalletAddress,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id": fmt.Sprintf("%d", runner.ID),
	}); err != nil {
		log := gologger.Get()
		log.Error().Err(err).Str("webhook_id", fmt.Sprintf("%d", runner.ID)).Msg("Failed to encode webhook registration response")
	}
}

func (h *TaskHandler) UnregisterWebhook(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["device_id"]
	if deviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	_, err := h.runnerService.UpdateRunner(r.Context(), &models.Runner{
		DeviceID: deviceID,
		Webhook:  "",
		Status:   models.RunnerStatusOffline,
	})

	if err != nil {
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
	log := gologger.Get()
	log.Info().Msg("Creating task")

	// Check content type for multipart form data
	contentType := r.Header.Get("Content-Type")
	var req CreateTaskRequest
	var dockerImage []byte

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form data
		if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
			log.Error().Err(err).Msg("Failed to parse multipart form")
			http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
			return
		}

		// Get task data from form
		taskData := r.FormValue("task")
		if taskData == "" {
			http.Error(w, "Task data is required", http.StatusBadRequest)
			return
		}

		log.Debug().Str("task_data", taskData).Msg("Received task data")

		if err := json.Unmarshal([]byte(taskData), &req); err != nil {
			log.Error().Err(err).Str("task_data", taskData).Msg("Failed to unmarshal task data")
			http.Error(w, "Invalid task data", http.StatusBadRequest)
			return
		}

		// Always set type to Docker for multipart requests
		req.Type = models.TaskTypeDocker

		// Get Docker image file if present
		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			log.Info().
				Str("filename", header.Filename).
				Int64("size", header.Size).
				Msg("Processing Docker image file")

			dockerImage, err = io.ReadAll(file)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read Docker image file")
				http.Error(w, "Failed to read Docker image file", http.StatusInternalServerError)
				return
			}

			// Upload Docker image to S3
			imageURL, err := h.s3Service.UploadDockerImage(r.Context(), dockerImage, strings.TrimSuffix(header.Filename, ".tar"))
			if err != nil {
				log.Error().Err(err).Msg("Failed to upload Docker image to S3")
				http.Error(w, "Failed to upload Docker image to S3", http.StatusInternalServerError)
				return
			}

			// Create Docker environment config if not present
			if req.Environment == nil {
				req.Environment = &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image":   req.Image,
						"command": req.Command,
					},
				}
			}

			// Create task config
			taskConfig := models.TaskConfig{
				Command:        req.Command,
				DockerImageURL: imageURL,
				ImageName:      strings.TrimSuffix(header.Filename, ".tar"),
			}

			// Marshal config
			var configErr error
			req.Config, configErr = json.Marshal(taskConfig)
			if configErr != nil {
				log.Error().Err(configErr).Msg("Failed to marshal task config")
				http.Error(w, "Failed to process task configuration", http.StatusInternalServerError)
				return
			}
		}
	} else {
		// Handle regular JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error().Err(err).Msg("Failed to decode request body")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// For JSON requests with Docker image
		if req.Image != "" {
			req.Type = models.TaskTypeDocker
			if req.Environment == nil {
				req.Environment = &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image":   req.Image,
						"command": req.Command,
					},
				}
			}

			taskConfig := models.TaskConfig{
				Command:   req.Command,
				ImageName: req.Image,
			}

			var configErr error
			req.Config, configErr = json.Marshal(taskConfig)
			if configErr != nil {
				log.Error().Err(configErr).Msg("Failed to marshal task config")
				http.Error(w, "Failed to process task configuration", http.StatusInternalServerError)
				return
			}
		}
	}

	log.Info().
		Interface("request", req).
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
		log.Error().Str("type", string(req.Type)).Msg("Invalid task type")
		http.Error(w, "Invalid task type", http.StatusBadRequest)
		return
	}

	if req.Type == models.TaskTypeDocker {
		if req.Environment == nil || req.Environment.Type != "docker" {
			http.Error(w, "Docker environment configuration is required", http.StatusBadRequest)
			return
		}

		// Validate Docker configuration
		var taskConfig models.TaskConfig
		if err := json.Unmarshal(req.Config, &taskConfig); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal task config")
			http.Error(w, "Invalid task configuration", http.StatusBadRequest)
			return
		}

		if taskConfig.ImageName == "" {
			http.Error(w, "Image name is required for Docker tasks", http.StatusBadRequest)
			return
		}

		if len(taskConfig.Command) == 0 {
			http.Error(w, "Command is required for Docker tasks", http.StatusBadRequest)
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
	log := gologger.WithComponent("task_start")

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

	// First get the task to check its current status
	task, err := h.service.GetTask(ctx, taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to get task")
		http.Error(w, fmt.Sprintf("Failed to get task: %v", err), http.StatusInternalServerError)
		return
	}

	// If task is already completed, return early
	if task.Status == models.TaskStatusCompleted {
		log.Warn().Str("task_id", taskID).Msg("Cannot start already completed task")
		http.Error(w, "Task is already completed", http.StatusConflict)
		return
	}

	// Check if task is already assigned to this runner
	runner, err := h.runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		log.Error().Err(err).Str("runner_id", runnerID).Msg("Failed to get runner")
		http.Error(w, fmt.Sprintf("Failed to get runner: %v", err), http.StatusInternalServerError)
		return
	}

	// If task is already running and assigned to this runner, we can proceed
	if task.Status == models.TaskStatusRunning {
		if runner.TaskID != nil && *runner.TaskID == task.ID {
			log.Info().
				Str("task_id", taskID).
				Str("runner_id", runnerID).
				Msg("Task already assigned to this runner and running")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			// Task is running but not by this runner
			log.Warn().
				Str("task_id", taskID).
				Str("runner_id", runnerID).
				Msg("Task is already running by another runner")
			http.Error(w, "Task is already assigned to a different runner", http.StatusConflict)
			return
		}
	}

	// Only try to assign if task is in pending state
	if task.Status == models.TaskStatusPending {
		if err := h.service.AssignTaskToRunner(ctx, taskID, runnerID); err != nil {
			if err.Error() == "task unavailable" {
				log.Warn().
					Str("task_id", taskID).
					Str("runner_id", runnerID).
					Msg("Task is unavailable for assignment")
				http.Error(w, "Task is unavailable for assignment", http.StatusConflict)
				return
			}
			log.Error().Err(err).
				Str("task_id", taskID).
				Str("runner_id", runnerID).
				Msg("Failed to assign task")
			http.Error(w, fmt.Sprintf("Failed to assign task: %v", err), http.StatusInternalServerError)
			return
		}

		// Get updated task after assignment
		task, err = h.service.GetTask(ctx, taskID)
		if err != nil {
			log.Error().Err(err).Str("task_id", taskID).Msg("Failed to get updated task")
			http.Error(w, fmt.Sprintf("Failed to get updated task: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Start the task if it's not already running
	if task.Status != models.TaskStatusRunning {
		if err := h.service.StartTask(ctx, taskID); err != nil {
			if err.Error() == "task already completed" {
				log.Warn().Str("task_id", taskID).Msg("Cannot start already completed task")
				http.Error(w, "Task is already completed", http.StatusConflict)
				return
			}

			log.Error().Err(err).Str("task_id", taskID).Msg("Failed to start task")
			http.Error(w, fmt.Sprintf("Failed to start task: %v", err), http.StatusInternalServerError)
			return
		}
	}

	log.Info().
		Str("task_id", taskID).
		Str("runner_id", runnerID).
		Msg("Task started successfully")

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

	info, err := h.stakeWallet.GetStakeInfo(task.CreatorDeviceID)
	if err != nil {
		return fmt.Errorf("failed to get stake info: %v", err)
	}

	if !info.Exists {
		return fmt.Errorf("creator device not registered - please stake first")
	}

	if info.Amount.Cmp(big.NewInt(0)) < 0 {
		return fmt.Errorf("no stake found - please stake some PRTY first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	doneCh := make(chan struct {
		info walletsdk.StakeInfo
		err  error
	})

	go func() {
		info, err := h.stakeWallet.GetStakeInfo(task.CreatorDeviceID)
		doneCh <- struct {
			info walletsdk.StakeInfo
			err  error
		}{info, err}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("stake check timed out: %v", ctx.Err())
	case result := <-doneCh:
		if result.err != nil {
			return fmt.Errorf("failed to get stake info: %v", result.err)
		}

		if !result.info.Exists {
			return fmt.Errorf("creator device not registered - please stake first")
		}

		if result.info.Amount.Cmp(big.NewInt(0)) < 0 {
			return fmt.Errorf("no stake found - please stake some PRTY first")
		}

		return nil
	}
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.GetTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
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
	ctx := r.Context()

	tasks, err := h.service.ListAvailableTasks(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *TaskHandler) NotifyRunnerOfTasks(runnerID string, tasks []*models.Task) error {
	log := gologger.WithComponent("task_handler")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner, err := h.runnerService.GetRunner(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to get runner: %w", err)
	}

	if runner.Webhook == "" {
		return fmt.Errorf("runner has no webhook URL")
	}

	// Skip if no tasks to send
	if len(tasks) == 0 {
		return nil
	}

	// Prepare webhook message
	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	message := WSMessage{
		Type:    "available_tasks",
		Payload: tasksJSON,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", runner.Webhook, bytes.NewBuffer(messageJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).
			Str("runner_id", runnerID).
			Msg("Failed to notify runner, will be handled on next heartbeat")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("runner_id", runnerID).
			Msg("Webhook notification failed, will be handled on next heartbeat")
		return nil
	}

	log.Info().
		Str("runner_id", runnerID).
		Int("num_tasks", len(tasks)).
		Msg("Successfully notified runner of tasks")
	return nil
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

// RegisterRunner handles runner registration with webhook URL
func (h *TaskHandler) RegisterRunner(w http.ResponseWriter, r *http.Request) {
	var registerRequest struct {
		DeviceID      string              `json:"device_id"`
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Webhook       string              `json:"webhook"`
	}

	log := gologger.WithComponent("task_handler")

	if err := json.NewDecoder(r.Body).Decode(&registerRequest); err != nil {
		log.Error().Err(err).Msg("Failed to decode register runner request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if registerRequest.DeviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	if registerRequest.WalletAddress == "" {
		http.Error(w, "Wallet address is required", http.StatusBadRequest)
		return
	}

	// Create or update runner in the database
	runner := &models.Runner{
		DeviceID:      registerRequest.DeviceID,
		WalletAddress: registerRequest.WalletAddress,
		Status:        registerRequest.Status,
		Webhook:       registerRequest.Webhook,
	}

	// If status is empty, set it to online
	if runner.Status == "" {
		runner.Status = models.RunnerStatusOnline
	}

	ctx := r.Context()
	savedRunner, err := h.runnerService.CreateOrUpdateRunner(ctx, runner)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register runner")
		http.Error(w, fmt.Sprintf("Failed to register runner: %v", err), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("device_id", runner.DeviceID).
		Str("wallet_address", runner.WalletAddress).
		Str("status", string(runner.Status)).
		Str("webhook", runner.Webhook).
		Msg("Runner registered successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(savedRunner); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// RunnerHeartbeat handles heartbeat messages from runners
func (h *TaskHandler) RunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	var message struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	log := gologger.WithComponent("task_handler")

	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		log.Error().Err(err).Msg("Failed to decode heartbeat message")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if message.Type != "heartbeat" {
		http.Error(w, "Invalid message type", http.StatusBadRequest)
		return
	}

	var heartbeat struct {
		DeviceID      string              `json:"device_id"`
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Timestamp     int64               `json:"timestamp"`
	}

	if err := json.Unmarshal(message.Payload, &heartbeat); err != nil {
		log.Error().Err(err).Msg("Failed to parse heartbeat payload")
		http.Error(w, "Invalid heartbeat payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if heartbeat.DeviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	// Update runner status in the database
	runner := &models.Runner{
		DeviceID:      heartbeat.DeviceID,
		WalletAddress: heartbeat.WalletAddress,
		Status:        heartbeat.Status,
	}

	ctx := r.Context()
	_, err := h.runnerService.UpdateRunnerStatus(ctx, runner)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update runner status")
		http.Error(w, fmt.Sprintf("Failed to update runner status: %v", err), http.StatusInternalServerError)
		return
	}

	log.Debug().
		Str("device_id", heartbeat.DeviceID).
		Str("status", string(heartbeat.Status)).
		Int64("timestamp", heartbeat.Timestamp).
		Msg("Runner heartbeat received")

	w.WriteHeader(http.StatusOK)
}
