package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// Environment represents a runtime environment for task execution
type Environment interface {
	Setup() error
	Run(task *Task) error
	Cleanup() error
	GetType() string
}

// EnvironmentConfig holds configuration for task environments
type EnvironmentConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

func (ec EnvironmentConfig) Value() (driver.Value, error) {
	return json.Marshal(ec)
}

func (ec *EnvironmentConfig) Scan(value interface{}) error {
	if value == nil {
		*ec = EnvironmentConfig{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, &ec)
}
