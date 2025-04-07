package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-server/internal/core/ports"
)

type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	DeviceID  string    `json:"-"` // Internal field, not exposed in JSON
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL           string `json:"url"`
	WalletAddress string `json:"wallet_address"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WebhookService struct {
	webhooks     map[string]WebhookRegistration
	webhookMutex sync.RWMutex
	taskService  ports.TaskServicer
	stopCh       chan struct{}
	taskUpdateCh chan struct{}
}

func NewWebhookService(taskService ports.TaskServicer) *WebhookService {
	return &WebhookService{
		webhooks:     make(map[string]WebhookRegistration),
		taskService:  taskService,
		taskUpdateCh: make(chan struct{}, 100),
	}
}

func (s *WebhookService) SetStopChannel(stopCh chan struct{}) {
	s.stopCh = stopCh
}

func (s *WebhookService) NotifyTaskUpdate() {
	select {
	case s.taskUpdateCh <- struct{}{}:
		go s.notifyWebhooks()
	case <-s.stopCh:
		return
	default:
	}
}

func (s *WebhookService) RegisterWebhook(req RegisterWebhookRequest, deviceID string) (string, error) {
	if req.URL == "" {
		return "", fmt.Errorf("webhook URL is required")
	}
	if deviceID == "" {
		return "", fmt.Errorf("X-Device-ID header is required")
	}

	webhookID := uuid.New().String()
	webhook := WebhookRegistration{
		ID:        webhookID,
		URL:       req.URL,
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
	}

	s.webhookMutex.Lock()
	s.webhooks[webhookID] = webhook
	s.webhookMutex.Unlock()

	log := gologger.WithComponent("webhook")
	log.Info().
		Str("webhook_id", webhookID).
		Str("device_id", deviceID).
		Msg("Webhook registered")

	go s.sendInitialNotification(webhook)

	return webhookID, nil
}

func (s *WebhookService) UnregisterWebhook(webhookID string) error {
	s.webhookMutex.Lock()
	webhook, exists := s.webhooks[webhookID]
	if !exists {
		s.webhookMutex.Unlock()
		return fmt.Errorf("webhook not found")
	}

	delete(s.webhooks, webhookID)
	s.webhookMutex.Unlock()

	log := gologger.WithComponent("webhook")
	log.Info().
		Str("webhook_id", webhookID).
		Str("device_id", webhook.DeviceID).
		Msg("Webhook unregistered")

	return nil
}

func (s *WebhookService) notifyWebhooks() {
	log := gologger.WithComponent("webhook")

	select {
	case <-s.stopCh:
		return
	default:
	}

	tasks, err := s.taskService.ListAvailableTasks(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for webhook notification")
		return
	}

	if len(tasks) == 0 {
		return
	}

	payload := WSMessage{
		Type:    "available_tasks",
		Payload: tasks,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal webhook payload")
		return
	}

	s.webhookMutex.RLock()
	webhooks := make([]WebhookRegistration, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		webhooks = append(webhooks, webhook)
	}
	s.webhookMutex.RUnlock()

	if len(webhooks) == 0 {
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, webhook := range webhooks {
		select {
		case <-s.stopCh:
			return
		default:
			sem <- struct{}{}
			wg.Add(1)

			go func(webhook WebhookRegistration) {
				defer func() {
					<-sem
					wg.Done()
				}()

				s.sendWebhookNotification(client, webhook, payloadBytes)
			}(webhook)
		}
	}

	wg.Wait()
}

func (s *WebhookService) sendWebhookNotification(client *http.Client, webhook WebhookRegistration, payloadBytes []byte) {
	log := gologger.WithComponent("webhook")

	req, err := http.NewRequest("POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		log.Error().Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to create webhook request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", webhook.ID)

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to send webhook notification")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Str("webhook_id", webhook.ID).
			Int("status", resp.StatusCode).
			Str("response", string(body)).
			Msg("Webhook notification failed")
		return
	}
}

func (s *WebhookService) sendInitialNotification(webhook WebhookRegistration) {
	log := gologger.WithComponent("webhook")

	tasks, err := s.taskService.ListAvailableTasks(context.Background())
	if err != nil {
		log.Error().Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to list tasks for initial notification")
		return
	}

	if len(tasks) == 0 {
		return
	}

	payload := WSMessage{
		Type:    "available_tasks",
		Payload: tasks,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to marshal initial notification payload")
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	s.sendWebhookNotification(client, webhook, payloadBytes)
}

func (s *WebhookService) CleanupResources() {
	s.webhookMutex.Lock()
	defer s.webhookMutex.Unlock()
	s.webhooks = make(map[string]WebhookRegistration)
}
