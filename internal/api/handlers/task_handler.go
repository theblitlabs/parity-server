package handlers

import (
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
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
	"github.com/theblitlabs/parity-server/internal/core/config"
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
	"github.com/theblitlabs/parity-server/internal/utils"
)

type TaskHandler struct {
	service             *services.TaskService
	storageService      services.StorageService
	stakeWallet         *walletsdk.StakeWallet
	webhookService      *services.WebhookService
	verificationService *services.VerificationService
	webhooks            map[string]requestmodels.WebhookRegistration
	config              *config.Config
}

func NewTaskHandler(service *services.TaskService, storageService services.StorageService, verificationService *services.VerificationService, cfg *config.Config) *TaskHandler {
	return &TaskHandler{
		service:             service,
		storageService:      storageService,
		verificationService: verificationService,
		webhooks:            make(map[string]requestmodels.WebhookRegistration),
		config:              cfg,
	}
}

func (h *TaskHandler) SetStakeWallet(wallet *walletsdk.StakeWallet) {
	h.stakeWallet = wallet
}

func (h *TaskHandler) SetWebhookService(service *services.WebhookService) {
	h.webhookService = service
}

func (h *TaskHandler) NotifyTaskUpdate() {
	if h.webhookService == nil {
		return
	}
	h.webhookService.NotifyTaskUpdate()
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
	var req requestmodels.CreateTaskRequest
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
			defer func() {
				if closeErr := f.Close(); closeErr != nil {
					log.Error().Err(closeErr).Msg("Failed to close Docker image file")
				}
			}()

			dockerImage, err = io.ReadAll(f)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read Docker image file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read Docker image file"})
				return
			}

			imageURL, err := h.storageService.UploadDockerImage(c.Request.Context(), dockerImage, strings.TrimSuffix(file.Filename, ".tar"))
			if err != nil {
				log.Error().Err(err).Msg("Failed to upload Docker image")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload Docker image"})
				return
			}

			if req.Environment == nil {
				config := map[string]interface{}{
					"image": req.Image,
				}

				req.Environment = &models.EnvironmentConfig{
					Type:   "docker",
					Config: config,
				}
			}

			taskConfig := models.TaskConfig{
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
				config := map[string]interface{}{
					"image": req.Image,
				}

				req.Environment = &models.EnvironmentConfig{
					Type:   "docker",
					Config: config,
				}
			}

			taskConfig := models.TaskConfig{
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

	creatorAddress := c.GetHeader("X-Creator-Address") // We store the creator address for reference, but don't require it now

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
		log.Error().Err(err).
			Str("device_id", deviceID).
			Msg("Stake validation failed")
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
	log := gologger.WithComponent("task_handler")

	if h.stakeWallet == nil {
		log.Error().Str("device_id", task.CreatorDeviceID).Msg("Stake wallet not initialized")
		return fmt.Errorf("stake wallet not initialized")
	}

	info, err := h.stakeWallet.GetStakeInfo(task.CreatorDeviceID)
	if err != nil {
		log.Error().Err(err).Str("device_id", task.CreatorDeviceID).Msg("Failed to get stake info")
		return fmt.Errorf("failed to get stake info: %v", err)
	}

	log.Info().
		Str("device_id", task.CreatorDeviceID).
		Bool("exists", info.Exists).
		Str("amount", info.Amount.String()).
		Msg("Retrieved stake info")

	if !info.Exists {
		log.Error().Str("device_id", task.CreatorDeviceID).Msg("Device is not registered in staking contract")
		return fmt.Errorf("device %s is not registered in the staking contract - please stake %s tokens first", task.CreatorDeviceID, h.getTokenSymbol())
	}

	minRequiredStake := big.NewInt(10)
	if info.Amount.Cmp(minRequiredStake) <= 0 {
		log.Error().
			Str("device_id", task.CreatorDeviceID).
			Str("current_balance", info.Amount.String()).
			Str("required_balance", minRequiredStake.String()).
			Msg("Insufficient stake balance")
		return fmt.Errorf("insufficient stake balance for device %s - current balance: %v %s, minimum required: %v %s",
			task.CreatorDeviceID,
			info.Amount.String(),
			h.getTokenSymbol(),
			minRequiredStake.String(),
			h.getTokenSymbol())
	}

	return nil
}

func (h *TaskHandler) getTokenSymbol() string {
	if h.config != nil && h.config.BlockchainNetwork.TokenSymbol != "" {
		return h.config.BlockchainNetwork.TokenSymbol
	}
	return "TOKEN" // Default fallback
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

	c.JSON(http.StatusOK, gin.H{"message": "task completed successfully"})
}

type VerificationRequest struct {
	TaskID              string `json:"task_id"`
	RunnerID            string `json:"runner_id"`
	ImageHashVerified   string `json:"image_hash_verified"`
	CommandHashVerified string `json:"command_hash_verified"`
	Timestamp           int64  `json:"timestamp"`
}

func (h *TaskHandler) VerifyTaskHashes(c *gin.Context) {
	log := gologger.WithComponent("task_handler")
	taskID := c.Param("id")

	if taskID == "" {
		log.Error().Msg("Task ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	var req VerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Invalid verification request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	runnerID := c.GetHeader("X-Runner-ID")
	if runnerID == "" {
		log.Error().Msg("Runner ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Runner ID is required"})
		return
	}

	if req.TaskID != taskID {
		log.Error().
			Str("url_task_id", taskID).
			Str("body_task_id", req.TaskID).
			Msg("Task ID mismatch")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID mismatch"})
		return
	}

	err := h.verificationService.VerifyTaskExecution(c.Request.Context(), taskID, req.ImageHashVerified, req.CommandHashVerified)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskID).
			Str("runner_id", runnerID).
			Msg("Hash verification failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hash verification failed", "details": err.Error()})
		return
	}

	log.Info().
		Str("task_id", taskID).
		Str("runner_id", runnerID).
		Str("image_hash", req.ImageHashVerified).
		Str("command_hash", req.CommandHashVerified).
		Msg("Hash verification successful")

	c.JSON(http.StatusOK, gin.H{"message": "Hash verification successful"})
}
