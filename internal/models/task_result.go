package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskResult struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TaskID          uuid.UUID `json:"task_id" gorm:"type:uuid;index;not null;constraint:fk_task,onDelete:CASCADE"`
	DeviceID        string    `json:"device_id" gorm:"type:varchar(255);not null"`
	DeviceIDHash    string    `json:"device_id_hash" gorm:"type:varchar(64);not null"`
	RunnerAddress   string    `json:"runner_address" gorm:"type:varchar(255);not null"`
	CreatorAddress  string    `json:"creator_address" gorm:"type:varchar(255);not null"`
	Output          string    `json:"output" gorm:"type:text"`
	Error           string    `json:"error,omitempty" gorm:"type:text"`
	ExitCode        int       `json:"exit_code" gorm:"type:int"`
	ExecutionTime   int64     `json:"execution_time" gorm:"type:bigint"`
	CreatedAt       time.Time `json:"created_at" gorm:"type:timestamp with time zone;default:now()"`
	CreatorDeviceID string    `json:"creator_device_id" gorm:"type:text"`
	SolverDeviceID  string    `json:"solver_device_id" gorm:"type:text"`
	Reward          float64   `json:"reward" gorm:"type:decimal(20,8)"`
	CPUSeconds      float64   `json:"cpu_seconds" gorm:"type:decimal(20,8);default:0"`
	EstimatedCycles uint64    `json:"estimated_cycles" gorm:"type:bigint;not null;default:0"`
	MemoryGBHours   float64   `json:"memory_gb_hours" gorm:"type:decimal(20,8);default:0"`
	StorageGB       float64   `json:"storage_gb" gorm:"type:decimal(20,8);default:0"`
	NetworkDataGB   float64   `json:"network_data_gb" gorm:"type:decimal(20,8);default:0"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
}

func (r *TaskResult) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

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

func NewTaskResult() *TaskResult {
	return &TaskResult{
		ID: uuid.New(),
	}
}
