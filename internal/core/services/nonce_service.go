package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/drand/drand/client"
	"github.com/drand/drand/client/http"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
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

	return hex.EncodeToString([]byte(fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String())))
}

func (s *NonceService) VerifyNonce(taskNonce string, taskOutput string) bool {
	if taskNonce == "" || taskOutput == "" {
		return false
	}

	log := gologger.WithComponent("nonce_service")
	log.Debug().
		Str("nonce", taskNonce).
		Str("output_length", fmt.Sprintf("%d", len(taskOutput))).
		Msg("Verifying nonce in task output")

	return strings.Contains(taskOutput, taskNonce)
}