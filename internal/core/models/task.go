package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type (
	TaskStatus string
	TaskType   string
)

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusRunning     TaskStatus = "running"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusFailed      TaskStatus = "failed"
	TaskStatusNotVerified TaskStatus = "not_verified"
)

const (
	TaskTypeDocker            TaskType = "docker"
	TaskTypeCommand           TaskType = "command"
	TaskTypeLLM               TaskType = "llm"
	TaskTypeFederatedLearning TaskType = "federated_learning"
)

type DockerConfig struct {
	Image   string   `json:"image"`
	Workdir string   `json:"workdir"`
	Command []string `json:"command,omitempty"`
}

type TaskConfig struct {
	FileURL        string            `json:"file_url,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Resources      ResourceConfig    `json:"resources,omitempty"`
	DockerImageURL string            `json:"docker_image_url,omitempty"`
	ImageName      string            `json:"image_name,omitempty"`
}

type ResourceConfig struct {
	Memory    string `json:"memory,omitempty"`
	CPUShares int64  `json:"cpu_shares,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

func (c *TaskConfig) Validate(taskType TaskType) error {
	switch taskType {
	case TaskTypeDocker:
		if c.ImageName == "" {
			return errors.New("image name is required for Docker tasks")
		}
		if c.DockerImageURL == "" && c.FileURL == "" {
			return errors.New("either docker image URL or file URL is required for Docker tasks")
		}
	case TaskTypeCommand:
		// Command can be empty for both Docker and Command tasks
	case TaskTypeLLM:
		// LLM tasks don't require Docker image or command validation
		// Validation for model and prompt is done at the environment level
	case TaskTypeFederatedLearning:
		// FL tasks have their own validation logic
		// No specific TaskConfig validation needed
	default:
		return fmt.Errorf("unsupported task type: %s", taskType)
	}
	return nil
}

type Task struct {
	ID              uuid.UUID          `json:"id" gorm:"type:uuid;primaryKey"`
	Title           string             `json:"title" gorm:"type:varchar(255)"`
	Description     string             `json:"description" gorm:"type:text"`
	Type            TaskType           `json:"type" gorm:"type:varchar(50)"`
	Status          TaskStatus         `json:"status" gorm:"type:varchar(50)"`
	Config          json.RawMessage    `json:"config" gorm:"type:jsonb"`
	Environment     *EnvironmentConfig `json:"environment" gorm:"type:jsonb"`
	Reward          float64            `json:"reward,omitempty" gorm:"type:decimal(20,8);default:0"`
	CreatorAddress  string             `json:"creator_address" gorm:"type:varchar(42)"`
	CreatorDeviceID string             `json:"creator_device_id" gorm:"type:varchar(255)"`
	RunnerID        string             `json:"runner_id" gorm:"type:varchar(255)"`
	Nonce           string             `json:"nonce" gorm:"type:varchar(64);not null"`
	ImageHash       string             `json:"image_hash" gorm:"type:varchar(64)"`
	CommandHash     string             `json:"command_hash" gorm:"type:varchar(64)"`
	CreatedAt       time.Time          `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt       time.Time          `json:"updated_at" gorm:"type:timestamp"`
	CompletedAt     *time.Time         `json:"completed_at" gorm:"type:timestamp"`
}

func NewTask() *Task {
	t := &Task{
		ID:        uuid.New(),
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return t
}

func (t *Task) Validate() error {
	if t.Title == "" {
		return errors.New("title is required")
	}

	if t.Type == "" {
		return errors.New("task type is required")
	}

	// Skip config validation for federated learning tasks as they have different structure
	if t.Type == TaskTypeFederatedLearning {
		// FL tasks have their own config format, just check that config exists
		if len(t.Config) == 0 {
			return errors.New("config is required for federated learning tasks")
		}
		return nil
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
