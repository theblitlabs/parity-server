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
	Title       string                         `json:"title"`
	Description string                         `json:"description"`
	Type        coremodels.TaskType           `json:"type"`
	Image       string                         `json:"image"`
	Command     []string                       `json:"command"`
	Config      json.RawMessage               `json:"config"`
	Environment *coremodels.EnvironmentConfig `json:"environment,omitempty"`
	Reward      float64                       `json:"reward"`
	CreatorID   string                        `json:"creator_id"`
} 