package services

import (
	"context"
	"encoding/hex"

	"github.com/drand/drand/client"
	"github.com/drand/drand/client/http"
	"github.com/theblitlabs/parity-server/internal/utils"
)

type NonceService struct {
	client client.Client
}

func NewNonceService() *NonceService {
	urls := []string{"https://api.drand.sh", "https://drand.cloudflare.com"}
	chainHash, _ := hex.DecodeString("8990e7a9aaed2ffed73dbd7092123d6f289930540d7651336225dc172e51b2ce")

	httpClients := http.ForURLs(urls, chainHash)
	if len(httpClients) == 0 {
		return &NonceService{}
	}

	c, err := client.New(
		client.From(httpClients...),
		client.WithChainHash(chainHash),
		client.WithCacheSize(0), // Disable caching for nonces
	)

	if err != nil {
		return &NonceService{}
	}

	return &NonceService{
		client: c,
	}
}

func (s *NonceService) GenerateNonce() string {
	if s.client != nil {
		result, err := s.client.Get(context.Background(), 0)
		if err == nil {
			return hex.EncodeToString(result.Randomness())
		}
	}

	return utils.GenerateNonce()
}

func (s *NonceService) VerifyNonce(taskNonce string, taskOutput string) bool {
	return utils.VerifyNonce(taskNonce, taskOutput)
}
