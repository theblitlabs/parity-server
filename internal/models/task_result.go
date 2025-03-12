package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskResult struct {
	ID              uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	TaskID          uuid.UUID       `json:"task_id" gorm:"type:uuid;index;not null;constraint:fk_task,onDelete:CASCADE"`
	DeviceID        string          `json:"device_id" gorm:"type:varchar(255);not null"`
	DeviceIDHash    string          `json:"device_id_hash" gorm:"type:varchar(64);not null"`
	RunnerAddress   string          `json:"runner_address" gorm:"type:varchar(255);not null"`
	CreatorAddress  string          `json:"creator_address" gorm:"type:varchar(255);not null"`
	Output          string          `json:"output" gorm:"type:text"`
	Error           string          `json:"error,omitempty" gorm:"type:text"`
	ExitCode        int             `json:"exit_code" gorm:"type:int"`
	ExecutionTime   int64           `json:"execution_time" gorm:"type:bigint"`
	CreatedAt       time.Time       `json:"created_at" gorm:"type:timestamp with time zone;default:now()"`
	CreatorDeviceID string          `json:"creator_device_id" gorm:"type:text"`
	SolverDeviceID  string          `json:"solver_device_id" gorm:"type:text"`
	Reward          float64         `json:"reward" gorm:"type:decimal(20,8)"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	IPFSCID         string          `json:"ipfs_cid" gorm:"type:text"`
	CPUSeconds      float64         `json:"cpu_seconds" gorm:"type:decimal(20,8);default:0"`
	EstimatedCycles uint64          `json:"estimated_cycles" gorm:"type:bigint;not null;default:0"`
	MemoryGBHours   float64         `json:"memory_gb_hours" gorm:"type:decimal(20,8);default:0"`
	StorageGB       float64         `json:"storage_gb" gorm:"type:decimal(20,8);default:0"`
	NetworkDataGB   float64         `json:"network_data_gb" gorm:"type:decimal(20,8);default:0"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
}

// GetMetadata returns the metadata as a map
func (r *TaskResult) GetMetadata() (map[string]interface{}, error) {
	if len(r.Metadata) == 0 {
		return make(map[string]interface{}), nil
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(r.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return metadata, nil
}

// SetMetadata sets the metadata from a map
func (r *TaskResult) SetMetadata(metadata map[string]interface{}) error {
	if metadata == nil {
		r.Metadata = json.RawMessage("{}")
		return nil
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	r.Metadata = data
	return nil
}

// BeforeCreate GORM hook to set ID and ensure metadata is valid JSON
func (r *TaskResult) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if len(r.Metadata) == 0 {
		r.Metadata = json.RawMessage("{}")
	}
	return nil
}

// Validate checks if all required fields are present and valid
func (r *TaskResult) Validate() error {
	if r.ID == uuid.Nil {
		return errors.New("result ID is required")
	}
	if r.TaskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if r.DeviceID == "" {
		return errors.New("device ID is required")
	}
	if r.DeviceIDHash == "" {
		return errors.New("device ID hash is required")
	}
	if r.RunnerAddress == "" {
		return errors.New("runner address is required")
	}
	if r.CreatorAddress == "" {
		return errors.New("creator address is required")
	}
	if r.CreatorDeviceID == "" {
		return errors.New("creator device ID is required")
	}
	if r.SolverDeviceID == "" {
		return errors.New("solver device ID is required")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("created at timestamp is required")
	}
	return nil
}

// NewTaskResult creates a new task result with a generated UUID
func NewTaskResult() *TaskResult {
	return &TaskResult{
		ID: uuid.New(),
	}
}
