package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type FederatedLearningRound struct {
	ID            uuid.UUID            `json:"id" gorm:"type:uuid;primaryKey"`
	SessionID     uuid.UUID            `json:"session_id" gorm:"type:uuid;not null"`
	RoundNumber   int                  `json:"round_number" gorm:"not null"`
	Status        FLRoundStatus        `json:"status" gorm:"type:varchar(50)"`
	ModelSnapshot json.RawMessage      `json:"model_snapshot" gorm:"type:jsonb"`
	Aggregation   *AggregationResult   `json:"aggregation" gorm:"type:jsonb"`
	Participants  []FLRoundParticipant `json:"participants" gorm:"-"`
	CreatedAt     time.Time            `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt     time.Time            `json:"updated_at" gorm:"type:timestamp"`
	CompletedAt   *time.Time           `json:"completed_at" gorm:"type:timestamp"`
}

type FederatedLearningSession struct {
	ID              uuid.UUID          `json:"id" gorm:"type:uuid;primaryKey"`
	Name            string             `json:"name" gorm:"type:varchar(255);not null"`
	Description     string             `json:"description" gorm:"type:text"`
	ModelType       string             `json:"model_type" gorm:"type:varchar(100);not null"`
	GlobalModel     json.RawMessage    `json:"global_model" gorm:"type:jsonb"`
	Status          FLSessionStatus    `json:"status" gorm:"type:varchar(50)"`
	Config          FLConfig           `json:"config" gorm:"type:jsonb"`
	TrainingData    TrainingDataConfig `json:"training_data" gorm:"type:jsonb"`
	CurrentRound    int                `json:"current_round" gorm:"default:0"`
	TotalRounds     int                `json:"total_rounds" gorm:"not null"`
	MinParticipants int                `json:"min_participants"`
	CreatorAddress  string             `json:"creator_address" gorm:"type:varchar(42);not null"`
	CreatedAt       time.Time          `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt       time.Time          `json:"updated_at" gorm:"type:timestamp"`
	CompletedAt     *time.Time         `json:"completed_at" gorm:"type:timestamp"`
}

type TrainingDataConfig struct {
	DatasetCID    string                 `json:"dataset_cid"`
	DatasetSize   int64                  `json:"dataset_size"`
	DataFormat    string                 `json:"data_format"`
	Features      []string               `json:"features,omitempty"`
	Labels        []string               `json:"labels,omitempty"`
	SplitStrategy string                 `json:"split_strategy"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Value implements the driver.Valuer interface for GORM
func (t TrainingDataConfig) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan implements the sql.Scanner interface for GORM
func (t *TrainingDataConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into TrainingDataConfig", value)
	}

	return json.Unmarshal(bytes, t)
}

type FLRoundParticipant struct {
	ID              uuid.UUID           `json:"id" gorm:"type:uuid;primaryKey"`
	RoundID         uuid.UUID           `json:"round_id" gorm:"type:uuid;not null"`
	RunnerID        string              `json:"runner_id" gorm:"type:varchar(255);not null"`
	ModelUpdate     json.RawMessage     `json:"model_update" gorm:"type:jsonb"`
	Status          FLParticipantStatus `json:"status" gorm:"type:varchar(50)"`
	Weight          float64             `json:"weight" gorm:"type:decimal(10,8);default:1.0"`
	DataSize        int                 `json:"data_size" gorm:"default:0"`
	TrainingMetrics json.RawMessage     `json:"training_metrics" gorm:"type:jsonb"`
	CreatedAt       time.Time           `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt       time.Time           `json:"updated_at" gorm:"type:timestamp"`
	CompletedAt     *time.Time          `json:"completed_at" gorm:"type:timestamp"`
}

type ModelUpdate struct {
	Gradients  map[string][]float64   `json:"gradients"`
	Weights    map[string][]float64   `json:"weights,omitempty"`
	UpdateType string                 `json:"update_type"`
	DataSize   int                    `json:"data_size"`
	Loss       float64                `json:"loss"`
	Accuracy   float64                `json:"accuracy,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type AggregationResult struct {
	Method           string               `json:"method"`
	AggregatedModel  map[string][]float64 `json:"aggregated_model"`
	Weights          map[string]float64   `json:"weights"`
	GlobalMetrics    GlobalMetrics        `json:"global_metrics"`
	RoundSummary     string               `json:"round_summary"`
	ParticipantCount int                  `json:"participant_count"`
}

type GlobalMetrics struct {
	AverageLoss     float64 `json:"average_loss"`
	AverageAccuracy float64 `json:"average_accuracy"`
	Variance        float64 `json:"variance"`
	Convergence     float64 `json:"convergence"`
}

type FLConfig struct {
	AggregationMethod string                 `json:"aggregation_method"`
	LearningRate      float64                `json:"learning_rate"`
	BatchSize         int                    `json:"batch_size"`
	LocalEpochs       int                    `json:"local_epochs"`
	ClientSelection   string                 `json:"client_selection"`
	ModelConfig       map[string]interface{} `json:"model_config"`
	PrivacyConfig     PrivacyConfig          `json:"privacy_config,omitempty"`
}

// Value implements the driver.Valuer interface for GORM
func (c FLConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface for GORM
func (c *FLConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into FLConfig", value)
	}

	return json.Unmarshal(bytes, c)
}

type PrivacyConfig struct {
	DifferentialPrivacy bool    `json:"differential_privacy"`
	NoiseMultiplier     float64 `json:"noise_multiplier,omitempty"`
	L2NormClip          float64 `json:"l2_norm_clip,omitempty"`
	SecureAggregation   bool    `json:"secure_aggregation"`
}

type (
	FLSessionStatus     string
	FLRoundStatus       string
	FLParticipantStatus string
)

const (
	FLSessionStatusPending   FLSessionStatus = "pending"
	FLSessionStatusActive    FLSessionStatus = "active"
	FLSessionStatusCompleted FLSessionStatus = "completed"
	FLSessionStatusFailed    FLSessionStatus = "failed"
)

const (
	FLRoundStatusPending     FLRoundStatus = "pending"
	FLRoundStatusCollecting  FLRoundStatus = "collecting"
	FLRoundStatusAggregating FLRoundStatus = "aggregating"
	FLRoundStatusCompleted   FLRoundStatus = "completed"
	FLRoundStatusFailed      FLRoundStatus = "failed"
)

const (
	FLParticipantStatusAssigned  FLParticipantStatus = "assigned"
	FLParticipantStatusTraining  FLParticipantStatus = "training"
	FLParticipantStatusCompleted FLParticipantStatus = "completed"
	FLParticipantStatusFailed    FLParticipantStatus = "failed"
)

func NewFederatedLearningSession(name, description, modelType, creatorAddress string, totalRounds, minParticipants int, config FLConfig, trainingData TrainingDataConfig) *FederatedLearningSession {
	return &FederatedLearningSession{
		ID:              uuid.New(),
		Name:            name,
		Description:     description,
		ModelType:       modelType,
		Status:          FLSessionStatusPending,
		Config:          config,
		TrainingData:    trainingData,
		CurrentRound:    0,
		TotalRounds:     totalRounds,
		MinParticipants: minParticipants,
		CreatorAddress:  creatorAddress,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func NewFederatedLearningRound(sessionID uuid.UUID, roundNumber int) *FederatedLearningRound {
	return &FederatedLearningRound{
		ID:          uuid.New(),
		SessionID:   sessionID,
		RoundNumber: roundNumber,
		Status:      FLRoundStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func NewFLRoundParticipant(roundID uuid.UUID, runnerID string) *FLRoundParticipant {
	return &FLRoundParticipant{
		ID:        uuid.New(),
		RoundID:   roundID,
		RunnerID:  runnerID,
		Status:    FLParticipantStatusAssigned,
		Weight:    1.0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
