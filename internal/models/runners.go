package models

import (
	"time"

	"github.com/google/uuid"
)

type Runner struct {
	ID            uint         `gorm:"primaryKey;autoIncrement"`
	DeviceID      string       `gorm:"type:varchar(255);unique"`
	WalletAddress string       `gorm:"type:varchar(42)"`
	Status        RunnerStatus `gorm:"type:varchar(255)"`
	Webhook       string       `gorm:"type:varchar(255)"`
	TaskID        *uuid.UUID   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Task          *Task        `gorm:"foreignKey:TaskID"`
	CreatedAt     time.Time    `gorm:"autoCreateTime"`
	UpdatedAt     time.Time    `gorm:"autoUpdateTime"`
}

type RunnerStatus string

const (
	RunnerStatusOnline  RunnerStatus = "online"
	RunnerStatusOffline RunnerStatus = "offline"
	RunnerStatusBusy    RunnerStatus = "busy"
)
