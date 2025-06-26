package models

import (
	"encoding/json"
	"time"

	coremodels "github.com/theblitlabs/parity-server/internal/core/models"
)

type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RunnerID  string    `json:"runner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL           string `json:"url"`
	WalletAddress string `json:"wallet_address"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type CreateTaskRequest struct {
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	Type        coremodels.TaskType           `json:"type"`
	Image       string                        `json:"image"`
	Config      json.RawMessage               `json:"config"`
	Environment *coremodels.EnvironmentConfig `json:"environment,omitempty"`
	Reward      float64                       `json:"reward"`
	CreatorID   string                        `json:"creator_id"`
}

type HeartbeatPayload struct {
	WalletAddress     string                  `json:"wallet_address"`
	Status            coremodels.RunnerStatus `json:"status"`
	Timestamp         int64                   `json:"timestamp"`
	Uptime            int64                   `json:"uptime"`
	Memory            int64                   `json:"memory_usage"`
	CPU               float64                 `json:"cpu_usage"`
	PublicIP          string                  `json:"public_ip,omitempty"`
	ModelCapabilities []ModelCapabilityInfo   `json:"model_capabilities,omitempty"`
}

type ModelCapabilityInfo struct {
	ModelName string `json:"model_name"`
	IsLoaded  bool   `json:"is_loaded"`
	MaxTokens int    `json:"max_tokens"`
}

type PromptRequest struct {
	Prompt    string `json:"prompt" binding:"required"`
	ModelName string `json:"model_name" binding:"required"`
}

type PromptResponse struct {
	ID          string  `json:"id"`
	Response    string  `json:"response"`
	Status      string  `json:"status"`
	ModelName   string  `json:"model_name"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type BillingMetricsResponse struct {
	TotalRequests    int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
	AvgInferenceTime float64 `json:"avg_inference_time_ms"`
}

type RegisterRunnerRequest struct {
	WalletAddress     string                `json:"wallet_address" binding:"required"`
	Webhook           string                `json:"webhook,omitempty"`
	ModelCapabilities []ModelCapabilityInfo `json:"model_capabilities,omitempty"`
}
