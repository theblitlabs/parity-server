package services

import (
	"context"

	"github.com/theblitlabs/parity-server/internal/core/config"
)

type StorageService interface {
	UploadDockerImage(ctx context.Context, imageData []byte, imageName string) (string, error)
	DeleteDockerImage(ctx context.Context, imageURL string) error
}

func NewStorageService(cfg *config.Config) (StorageService, error) {
	return NewFilecoinService(cfg)
}
