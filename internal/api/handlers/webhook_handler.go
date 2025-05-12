package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/api/models"
	coremodels "github.com/theblitlabs/parity-server/internal/core/models"
	"github.com/theblitlabs/parity-server/internal/core/services"
)

type WebhookHandler struct {
	webhookService *services.WebhookService
	runnerService  *services.RunnerService
	stopCh         chan struct{}
}

func NewWebhookHandler(webhookService *services.WebhookService, runnerService *services.RunnerService) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		runnerService:  runnerService,
	}
}

func (h *WebhookHandler) SetStopChannel(stopCh chan struct{}) {
	h.stopCh = stopCh
	h.webhookService.SetStopChannel(stopCh)
}

func (h *WebhookHandler) RegisterWebhook(c *gin.Context) {
	log := gologger.WithComponent("webhook_handler")
	var req models.RegisterWebhookRequest
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

	_, err := h.runnerService.CreateOrUpdateRunner(c.Request.Context(), &coremodels.Runner{
		DeviceID:      deviceID,
		Status:        coremodels.RunnerStatusOnline,
		Webhook:       req.URL,
		WalletAddress: req.WalletAddress,
	})
	if err != nil {
		log.Error().Err(err).Str("device_id", deviceID).Msg("Failed to create/update runner")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	serviceReq := services.RegisterWebhookRequest{
		URL:           req.URL,
		WalletAddress: req.WalletAddress,
	}

	webhookID, err := h.webhookService.RegisterWebhook(serviceReq, deviceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register webhook")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": webhookID,
	})
}

func (h *WebhookHandler) UnregisterWebhook(c *gin.Context) {
	deviceID := c.GetHeader("X-Device-ID")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Device-ID header is required"})
		return
	}

	_, err := h.runnerService.UpdateRunner(c.Request.Context(), &coremodels.Runner{
		DeviceID: deviceID,
		Webhook:  "",
		Status:   coremodels.RunnerStatusOffline,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (h *WebhookHandler) CleanupResources() {
	h.webhookService.CleanupResources()
}
