package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string
type TaskType string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

const (
	TaskTypeDocker  TaskType = "docker"
	TaskTypeCommand TaskType = "command"
)

type TaskConfig struct {
	FileURL   string            `json:"file_url,omitempty"`
	Command   []string          `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Resources ResourceConfig    `json:"resources,omitempty"`
}

func (c *TaskConfig) Validate(taskType TaskType) error {
	switch taskType {
	case TaskTypeDocker:
		if len(c.Command) == 0 {
			return errors.New("command is required for Docker tasks")
		}
	case TaskTypeCommand:
		if len(c.Command) == 0 {
			return errors.New("command is required for Command tasks")
		}
	default:
		return fmt.Errorf("unsupported task type: %s", taskType)
	}
	return nil
}

type ResourceConfig struct {
	Memory    string `json:"memory,omitempty"`
	CPUShares int64  `json:"cpu_shares,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

type Task struct {
	ID              uuid.UUID          `json:"id" gorm:"type:uuid;primaryKey"`
	Title           string             `json:"title" gorm:"type:varchar(255)"`
	Description     string             `json:"description" gorm:"type:text"`
	Type            TaskType           `json:"type" gorm:"type:varchar(50)"`
	Status          TaskStatus         `json:"status" gorm:"type:varchar(50)"`
	Config          json.RawMessage    `json:"config" gorm:"type:jsonb"`
	Environment     *EnvironmentConfig `json:"environment" gorm:"type:jsonb"`
	Reward          *float64           `json:"reward,omitempty" gorm:"type:decimal(20,8)"`
	CreatorID       uuid.UUID          `json:"creator_id" gorm:"type:uuid;not null"`
	CreatorAddress  string             `json:"creator_address" gorm:"type:varchar(42)"`
	CreatorDeviceID string             `json:"creator_device_id" gorm:"type:varchar(255)"`
	RunnerID        *uuid.UUID         `json:"runner_id" gorm:"type:uuid"`
	CreatedAt       time.Time          `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt       time.Time          `json:"updated_at" gorm:"type:timestamp"`
	CompletedAt     *time.Time         `json:"completed_at" gorm:"type:timestamp"`
}

// NewTask creates a new Task with a generated UUID
func NewTask() *Task {
	return &Task{
		ID:        uuid.New(),
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate performs basic validation on the task
func (t *Task) Validate() error {
	if t.Title == "" {
		return errors.New("title is required")
	}

	if t.Type == "" {
		return errors.New("task type is required")
	}

	var config TaskConfig
	if err := json.Unmarshal(t.Config, &config); err != nil {
		return fmt.Errorf("failed to unmarshal task config: %w", err)
	}

	if err := config.Validate(t.Type); err != nil {
		return err
	}

	if t.Type == TaskTypeDocker && (t.Environment == nil || t.Environment.Type != "docker") {
		return errors.New("docker environment configuration is required for docker tasks")
	}

	return nil
}
