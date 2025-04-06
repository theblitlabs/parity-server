package models

import (
	"time"

	"github.com/google/uuid"
)

type Runner struct {
	ID            uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	DeviceID      string       `json:"device_id" gorm:"type:varchar(255);unique"`
	WalletAddress string       `json:"wallet_address" gorm:"type:varchar(42)"`
	Status        RunnerStatus `json:"status" gorm:"type:varchar(255)"`
	Webhook       string       `json:"webhook" gorm:"type:varchar(255)"`
	TaskID        *uuid.UUID   `json:"task_id,omitempty" gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Task          *Task        `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	LastHeartbeat time.Time    `json:"last_heartbeat" gorm:"type:timestamp;default:now()"`
	CreatedAt     time.Time    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time    `json:"updated_at" gorm:"autoUpdateTime"`
}

type RunnerStatus string

const (
	RunnerStatusOnline  RunnerStatus = "online"
	RunnerStatusOffline RunnerStatus = "offline"
	RunnerStatusBusy    RunnerStatus = "busy"
)
