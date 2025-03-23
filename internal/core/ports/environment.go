package ports

import (
	"github.com/theblitlabs/parity-server/internal/core/models"
)

type Environment interface {
	Setup() error
	Run(task *models.Task) error
	Cleanup() error
	GetType() string
}
