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
	Prompt         string `json:"prompt" binding:"required"`
	ModelName      string `json:"model_name" binding:"required"`
	CreatorAddress string `json:"creator_address" binding:"required"`
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

type CreateFLSessionRequest struct {
	Name            string           `json:"name" binding:"required"`
	Description     string           `json:"description"`
	ModelType       string           `json:"model_type" binding:"required"`
	TotalRounds     int              `json:"total_rounds" binding:"required"`
	MinParticipants int              `json:"min_participants"`
	Config          FLConfigRequest  `json:"config" binding:"required"`
	CreatorAddress  string           `json:"creator_address" binding:"required"`
	TrainingData    TrainingDataInfo `json:"training_data" binding:"required"`
}

type TrainingDataInfo struct {
	DatasetCID    string                 `json:"dataset_cid" binding:"required"`
	DatasetSize   int64                  `json:"dataset_size"`
	DataFormat    string                 `json:"data_format"`
	Features      []string               `json:"features,omitempty"`
	Labels        []string               `json:"labels,omitempty"`
	SplitStrategy string                 `json:"split_strategy"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type FLConfigRequest struct {
	AggregationMethod string                 `json:"aggregation_method"`
	LearningRate      float64                `json:"learning_rate"`
	BatchSize         int                    `json:"batch_size"`
	LocalEpochs       int                    `json:"local_epochs"`
	ClientSelection   string                 `json:"client_selection"`
	ModelConfig       map[string]interface{} `json:"model_config"`
	PrivacyConfig     PrivacyConfigRequest   `json:"privacy_config,omitempty"`
}

type PrivacyConfigRequest struct {
	DifferentialPrivacy bool    `json:"differential_privacy"`
	NoiseMultiplier     float64 `json:"noise_multiplier,omitempty"`
	L2NormClip          float64 `json:"l2_norm_clip,omitempty"`
	SecureAggregation   bool    `json:"secure_aggregation"`
}

type JoinFLSessionRequest struct {
	SessionID     string `json:"session_id" binding:"required"`
	RunnerID      string `json:"runner_id" binding:"required"`
	WalletAddress string `json:"wallet_address" binding:"required"`
}

type SubmitModelUpdateRequest struct {
	SessionID      string                 `json:"session_id" binding:"required"`
	RoundID        string                 `json:"round_id" binding:"required"`
	RunnerID       string                 `json:"runner_id" binding:"required"`
	Gradients      map[string][]float64   `json:"gradients" binding:"required"`
	Weights        map[string][]float64   `json:"weights,omitempty"`
	UpdateType     string                 `json:"update_type"`
	DataSize       int                    `json:"data_size"`
	Loss           float64                `json:"loss"`
	Accuracy       float64                `json:"accuracy,omitempty"`
	TrainingTime   int64                  `json:"training_time_ms"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	PrivacyMetrics *PrivacyMetricsRequest `json:"privacy_metrics,omitempty"`
}

type PrivacyMetricsRequest struct {
	NoiseScale      float64 `json:"noise_scale,omitempty"`
	ClippingApplied bool    `json:"clipping_applied,omitempty"`
	EpsilonUsed     float64 `json:"epsilon_used,omitempty"`
}

type FLSessionResponse struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	ModelType        string          `json:"model_type"`
	Status           string          `json:"status"`
	CurrentRound     int             `json:"current_round"`
	TotalRounds      int             `json:"total_rounds"`
	MinParticipants  int             `json:"min_participants"`
	ParticipantCount int             `json:"participant_count"`
	Config           FLConfigRequest `json:"config"`
	CreatorAddress   string          `json:"creator_address"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
	CompletedAt      *string         `json:"completed_at,omitempty"`
}

type FLRoundResponse struct {
	ID            string                  `json:"id"`
	SessionID     string                  `json:"session_id"`
	RoundNumber   int                     `json:"round_number"`
	Status        string                  `json:"status"`
	Participants  []FLParticipantResponse `json:"participants"`
	GlobalMetrics *GlobalMetricsResponse  `json:"global_metrics,omitempty"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
	CompletedAt   *string                 `json:"completed_at,omitempty"`
}

type FLParticipantResponse struct {
	ID              string                 `json:"id"`
	RoundID         string                 `json:"round_id"`
	RunnerID        string                 `json:"runner_id"`
	Status          string                 `json:"status"`
	Weight          float64                `json:"weight"`
	DataSize        int                    `json:"data_size"`
	TrainingMetrics map[string]interface{} `json:"training_metrics,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	CompletedAt     *string                `json:"completed_at,omitempty"`
}

type GlobalMetricsResponse struct {
	AverageLoss     float64 `json:"average_loss"`
	AverageAccuracy float64 `json:"average_accuracy"`
	Variance        float64 `json:"variance"`
	Convergence     float64 `json:"convergence"`
}
