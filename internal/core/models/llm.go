package models

import (
	"time"

	"github.com/google/uuid"
)

type PromptRequest struct {
	ID          uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey"`
	ClientID    string       `json:"client_id" gorm:"type:varchar(255);not null"`
	Prompt      string       `json:"prompt" gorm:"type:text;not null"`
	ModelName   string       `json:"model_name" gorm:"type:varchar(100);not null"`
	RunnerID    string       `json:"runner_id" gorm:"type:varchar(255)"`
	Status      PromptStatus `json:"status" gorm:"type:varchar(50);default:'pending'"`
	Response    string       `json:"response" gorm:"type:text"`
	CreatedAt   time.Time    `json:"created_at" gorm:"autoCreateTime"`
	CompletedAt *time.Time   `json:"completed_at,omitempty" gorm:"type:timestamp"`
}

type PromptStatus string

const (
	PromptStatusPending    PromptStatus = "pending"
	PromptStatusProcessing PromptStatus = "processing"
	PromptStatusCompleted  PromptStatus = "completed"
	PromptStatusFailed     PromptStatus = "failed"
)

type ModelCapability struct {
	ID        uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	RunnerID  string     `json:"runner_id" gorm:"type:varchar(255);not null"`
	ModelName string     `json:"model_name" gorm:"type:varchar(100);not null"`
	IsLoaded  bool       `json:"is_loaded" gorm:"default:false"`
	MaxTokens int        `json:"max_tokens" gorm:"default:4096"`
	LoadedAt  *time.Time `json:"loaded_at,omitempty" gorm:"type:timestamp"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

type BillingMetric struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ClientID       string    `json:"client_id" gorm:"type:varchar(255);not null"`
	PromptID       uuid.UUID `json:"prompt_id" gorm:"type:uuid;not null"`
	ModelName      string    `json:"model_name" gorm:"type:varchar(100);not null"`
	PromptTokens   int       `json:"prompt_tokens" gorm:"not null"`
	ResponseTokens int       `json:"response_tokens" gorm:"not null"`
	TotalTokens    int       `json:"total_tokens" gorm:"not null"`
	InferenceTime  int64     `json:"inference_time_ms" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func NewPromptRequest(clientID, prompt, modelName string) *PromptRequest {
	return &PromptRequest{
		ID:        uuid.New(),
		ClientID:  clientID,
		Prompt:    prompt,
		ModelName: modelName,
		Status:    PromptStatusPending,
		CreatedAt: time.Now(),
	}
}

func NewBillingMetric(clientID string, promptID uuid.UUID, modelName string, promptTokens, responseTokens int, inferenceTime int64) *BillingMetric {
	return &BillingMetric{
		ID:             uuid.New(),
		ClientID:       clientID,
		PromptID:       promptID,
		ModelName:      modelName,
		PromptTokens:   promptTokens,
		ResponseTokens: responseTokens,
		TotalTokens:    promptTokens + responseTokens,
		InferenceTime:  inferenceTime,
		CreatedAt:      time.Now(),
	}
}
