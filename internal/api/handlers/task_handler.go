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
	"github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
	"github.com/theblitlabs/parity-server/internal/utils"
	requestmodels "github.com/theblitlabs/parity-server/internal/api/models"
)

type TaskHandler struct {
	service     *services.TaskService
	s3Service   *services.S3Service
	stakeWallet *walletsdk.StakeWallet
	webhookService *services.WebhookService
	webhooks       map[string]requestmodels.WebhookRegistration
	stopCh         chan struct{}
	runnerService  *services.RunnerService
}

func NewTaskHandler(service *services.TaskService, s3Service *services.S3Service) *TaskHandler {
	return &TaskHandler{
		service:   service,
		s3Service: s3Service,
		webhooks:  make(map[string]requestmodels.WebhookRegistration),
	}
}

func (h *TaskHandler) SetStakeWallet(wallet *walletsdk.StakeWallet) {
	h.stakeWallet = wallet
}

func (h *TaskHandler) NotifyTaskUpdate() {
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
		log.Error().Err(err).
			Str("device_id", deviceID).
			Str("creator_address", task.CreatorAddress).
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
	if h.stakeWallet == nil {
		return fmt.Errorf("stake wallet not initialized")
	}

	info, err := h.stakeWallet.GetStakeInfo(task.CreatorAddress)
	if err != nil {
		return fmt.Errorf("failed to get stake info: %v", err)
	}

	if !info.Exists {
		return fmt.Errorf("wallet %s is not registered in the staking contract - please stake PRTY tokens first", task.CreatorAddress)
	}

	minRequiredStake := big.NewInt(10)
	if info.Amount.Cmp(minRequiredStake) <= 0 {
		return fmt.Errorf("insufficient stake balance for wallet %s - current balance: %v PRTY, minimum required: %v PRTY",
			task.CreatorAddress,
			info.Amount.String(),
			minRequiredStake.String())
	}

	return nil
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

	task, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}