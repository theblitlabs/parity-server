package models

import (
	"database/sql/driver"
	"encoding/json"
)

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
	return json.Unmarshal(value.([]byte), ec)
}
