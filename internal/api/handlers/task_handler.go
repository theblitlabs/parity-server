package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/ports"
	"github.com/theblitlabs/parity-server/internal/core/services"
	"github.com/theblitlabs/parity-server/internal/utils"
)

type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RunnerID  string    `json:"runner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL string `json:"url"`
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

type TaskHandler struct {
	service        ports.TaskService
	webhookService *services.WebhookService
	s3Service      *services.S3Service
	stakeWallet    *walletsdk.StakeWallet
	webhooks       map[string]WebhookRegistration
	stopCh         chan struct{}
	runnerService  *services.RunnerService
}

func NewTaskHandler(service ports.TaskService, webhookService *services.WebhookService, runnerService *services.RunnerService, s3Service *services.S3Service) *TaskHandler {
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

func (h *TaskHandler) RegisterWebhook(c *gin.Context) {
	log := gologger.WithComponent("task_handler")
	var req services.RegisterWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("X-Device-ID header is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Device-ID header is required"})
		return
	}

	_, err := h.runnerService.CreateOrUpdateRunner(c.Request.Context(), &models.Runner{
		DeviceID:      deviceID,
		Status:        models.RunnerStatusOnline,
		Webhook:       req.URL,
		WalletAddress: req.WalletAddress,
	})
	if err != nil {
		log.Error().Err(err).Str("device_id", deviceID).Msg("Failed to create/update runner")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhookID, err := h.webhookService.RegisterWebhook(req, deviceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register webhook")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": webhookID,
	})
}

func (h *TaskHandler) UnregisterWebhook(c *gin.Context) {
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Device-ID header is required"})
		return
	}

	_, err := h.runnerService.UpdateRunner(c.Request.Context(), &models.Runner{
		DeviceID: deviceID,
		Webhook:  "",
		Status:   models.RunnerStatusOffline,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (h *TaskHandler) GetTaskResult(c *gin.Context) {
	log := gologger.Get()
	taskID := c.Param("id")
	if taskID == "" {
		log.Error().Msg("Task ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	result, err := h.service.GetTaskResult(c.Request.Context(), taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to get task result")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result == nil {
		log.Error().Str("task_id", taskID).Msg("Task result not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "task result not found"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	log := gologger.WithComponent("task_handler")
	contentType := c.GetHeader("Content-Type")
	var req CreateTaskRequest
	var dockerImage []byte

	if strings.HasPrefix(contentType, "multipart/form-data") {
		form, err := c.MultipartForm()
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse multipart form")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
			return
		}

		taskData := form.Value["task"]
		if len(taskData) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Task data is required"})
			return
		}

		if err := json.Unmarshal([]byte(taskData[0]), &req); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal task data")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task data"})
			return
		}

		req.Type = models.TaskTypeDocker

		file, err := c.FormFile("image")
		if err == nil {
			f, err := file.Open()
			if err != nil {
				log.Error().Err(err).Msg("Failed to open Docker image file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open Docker image file"})
				return
			}
			defer f.Close()

			dockerImage, err = io.ReadAll(f)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read Docker image file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read Docker image file"})
				return
			}

			imageURL, err := h.s3Service.UploadDockerImage(c.Request.Context(), dockerImage, strings.TrimSuffix(file.Filename, ".tar"))
			if err != nil {
				log.Error().Err(err).Msg("Failed to upload Docker image")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload Docker image"})
				return
			}

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
				Command:        req.Command,
				DockerImageURL: imageURL,
				ImageName:      strings.TrimSuffix(file.Filename, ".tar"),
			}

			var configErr error
			req.Config, configErr = json.Marshal(taskConfig)
			if configErr != nil {
				log.Error().Err(configErr).Msg("Failed to marshal task config")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process task configuration"})
				return
			}
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process task configuration"})
				return
			}
		}
	}

	if req.Title == "" || req.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title and description are required"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	creatorAddress := c.GetHeader("X-Creator-Address")
	if creatorAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Creator address is required"})
		return
	}

	if req.Type != models.TaskTypeDocker && req.Type != models.TaskTypeCommand {
		log.Error().Str("type", string(req.Type)).Msg("Invalid task type")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task type"})
		return
	}

	if req.Type == models.TaskTypeDocker {
		if req.Environment == nil || req.Environment.Type != "docker" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Docker environment configuration is required"})
			return
		}

		var taskConfig models.TaskConfig
		if err := json.Unmarshal(req.Config, &taskConfig); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal task config")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task configuration"})
			return
		}

		if taskConfig.ImageName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Image name is required for Docker tasks"})
			return
		}

		if len(taskConfig.Command) == 0 {
			taskConfig.Command = []string{}
		}
	}

	nonce := utils.GenerateNonce()
	task := models.NewTask()
	task.Title = req.Title
	task.Description = req.Description
	task.Type = req.Type
	task.Config = req.Config
	task.Environment = req.Environment
	task.CreatorDeviceID = deviceID
	task.CreatorAddress = creatorAddress
	task.Nonce = nonce

	if err := h.checkStakeBalance(task); err != nil {
		log.Error().Err(err).Str("device_id", deviceID).Msg("Insufficient stake balance")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateTask(c.Request.Context(), task); err != nil {
		log.Error().Err(err).Msg("Failed to create task")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.NotifyTaskUpdate()

	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) StartTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	if err := h.service.AssignTaskToRunner(c.Request.Context(), taskID, deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.StartTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) SaveTaskResult(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	var result models.TaskResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result.TaskID = taskUUID
	result.DeviceID = deviceID
	result.CreatorDeviceID = task.CreatorDeviceID
	result.SolverDeviceID = deviceID
	result.CreatorAddress = task.CreatorAddress
	if result.CreatorAddress == "" {
		result.CreatorAddress = task.CreatorDeviceID
	}
	result.RunnerAddress = deviceID
	result.CreatedAt = time.Now()
	result.DeviceIDHash = utils.HashDeviceID(deviceID)
	result.Clean()

	if err := h.service.SaveTaskResult(c.Request.Context(), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
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

func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := h.service.GetTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) AssignTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	var req struct {
		RunnerID string `json:"runner_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.RunnerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Runner ID is required"})
		return
	}

	if err := h.service.AssignTaskToRunner(c.Request.Context(), taskID, req.RunnerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) GetTaskReward(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	reward, err := h.service.GetTaskReward(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reward": reward})
}

func (h *TaskHandler) ListAvailableTasks(c *gin.Context) {
	tasks, err := h.service.ListAvailableTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
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

	if len(tasks) == 0 {
		return nil
	}

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

func (h *TaskHandler) CompleteTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	if err := h.service.CompleteTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) CleanupResources() {
	h.webhookService.CleanupResources()
}

func (h *TaskHandler) RegisterRunner(c *gin.Context) {
	var runner models.Runner
	log := gologger.WithComponent("task_handler")

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	log.Info().Str("raw_body", string(rawBody)).Msg("Incoming request body")

	if err := c.ShouldBindJSON(&runner); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	log.Info().Fields(map[string]interface{}{
		"wallet_address": runner.WalletAddress,
		"webhook":        runner.Webhook,
		"status":         runner.Status,
	}).Msg("Parsed request body")

	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("X-Device-ID header is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	if runner.WalletAddress == "" {
		log.Error().Msg("Wallet address is missing in request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wallet address is required"})
		return
	}

	runner.Status = models.RunnerStatusOnline
	runner.DeviceID = deviceID

	createdRunner, err := h.runnerService.CreateOrUpdateRunner(c.Request.Context(), &runner)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create/update runner")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().Fields(map[string]interface{}{
		"device_id":      createdRunner.DeviceID,
		"wallet_address": createdRunner.WalletAddress,
		"status":         createdRunner.Status,
		"webhook":        createdRunner.Webhook,
	}).Msg("Runner created/updated successfully")

	c.JSON(http.StatusCreated, createdRunner)
}

func (h *TaskHandler) RunnerHeartbeat(c *gin.Context) {
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	runner := &models.Runner{
		DeviceID: deviceID,
		Status:   models.RunnerStatusOnline,
	}

	if _, err := h.runnerService.UpdateRunnerStatus(c.Request.Context(), runner); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
