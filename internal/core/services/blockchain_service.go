package services

import (
	"bytes"
	"context"
	"fmt"
	"path"

	"github.com/google/uuid"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/config"
)

type BlockchainService struct {
	ipfsClient  *shell.Shell
	gatewayURL  string
	storageDeal bool
}

func NewBlockchainService(cfg *config.Config) (*BlockchainService, error) {
	if cfg.Blockchain.IPFSEndpoint == "" {
		return nil, fmt.Errorf("ipfs endpoint must be specified")
	}

	if cfg.Blockchain.GatewayURL == "" {
		return nil, fmt.Errorf("blockchain gateway URL must be specified")
	}

	ipfsClient := shell.NewShell(cfg.Blockchain.IPFSEndpoint)

	return &BlockchainService{
		ipfsClient:  ipfsClient,
		gatewayURL:  cfg.Blockchain.GatewayURL,
		storageDeal: cfg.Blockchain.CreateStorageDeals,
	}, nil
}

func (b *BlockchainService) UploadDockerImage(ctx context.Context, imageData []byte, imageName string) (string, error) {
	log := gologger.Get()

	filename := fmt.Sprintf("%s-%s%s", imageName, uuid.New().String(), ".tar")

	log.Info().
		Str("filename", filename).
		Int("size", len(imageData)).
		Msg("Uploading Docker image to IPFS")

	reader := bytes.NewReader(imageData)

	cid, err := b.ipfsClient.Add(reader, shell.Pin(true))
	if err != nil {
		log.Error().Err(err).
			Str("filename", filename).
			Msg("Failed to upload Docker image to IPFS")
		return "", fmt.Errorf("failed to upload Docker image to IPFS: %w", err)
	}

	if b.storageDeal {
		if err := b.createStorageDeal(cid); err != nil {
			log.Warn().Err(err).
				Str("cid", cid).
				Msg("Failed to create storage deal, image stored on IPFS only")
		}
	}

	retrievalURL := fmt.Sprintf("%s/ipfs/%s", b.gatewayURL, cid)

	log.Info().
		Str("filename", filename).
		Str("cid", cid).
		Str("url", retrievalURL).
		Msg("Successfully uploaded Docker image to Blockchain/IPFS")

	return retrievalURL, nil
}

func (b *BlockchainService) DeleteDockerImage(ctx context.Context, imageURL string) error {
	log := gologger.Get()

	cid := path.Base(imageURL)

	err := b.ipfsClient.Unpin(cid)
	if err != nil {
		log.Error().Err(err).
			Str("cid", cid).
			Msg("Failed to unpin Docker image from IPFS")
		return fmt.Errorf("failed to unpin Docker image: %w", err)
	}

	log.Info().
		Str("cid", cid).
		Msg("Successfully unpinned Docker image from IPFS")

	return nil
}

func (b *BlockchainService) GetImageURL(cid string) string {
	return fmt.Sprintf("%s/ipfs/%s", b.gatewayURL, cid)
}

func (b *BlockchainService) createStorageDeal(cid string) error {
	log := gologger.Get()

	log.Info().
		Str("cid", cid).
		Msg("Creating storage deal for Docker image")

	return nil
}
