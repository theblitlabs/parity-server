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
)

type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RunnerID  string    `json:"runner_id"`
	DeviceID  string    `json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL      string `json:"url"`
	RunnerID string `json:"runner_id"`
	DeviceID string `json:"device_id"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WebhookService struct {
	webhooks     map[string]WebhookRegistration
	webhookMutex sync.RWMutex
	taskService  TaskService
	stopCh       chan struct{}
	taskUpdateCh chan struct{}
}

func NewWebhookService(taskService TaskService) *WebhookService {
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
	log := gologger.Get()
	select {
	case s.taskUpdateCh <- struct{}{}:
		go s.notifyWebhooks()
	case <-s.stopCh:
		log.Debug().Msg("NotifyTaskUpdate: Ignoring update during shutdown")
	default:
	}
}

func (s *WebhookService) RegisterWebhook(req RegisterWebhookRequest) (string, error) {
	if req.URL == "" {
		return "", fmt.Errorf("webhook URL is required")
	}
	if req.RunnerID == "" {
		return "", fmt.Errorf("runner ID is required")
	}
	if req.DeviceID == "" {
		return "", fmt.Errorf("device ID is required")
	}

	webhookID := uuid.New().String()
	webhook := WebhookRegistration{
		ID:        webhookID,
		URL:       req.URL,
		RunnerID:  req.RunnerID,
		DeviceID:  req.DeviceID,
		CreatedAt: time.Now(),
	}

	s.webhookMutex.Lock()
	s.webhooks[webhookID] = webhook
	s.webhookMutex.Unlock()

	log := gologger.WithComponent("webhook")
	log.Info().
		Str("webhook_id", webhookID).
		Str("url", req.URL).
		Str("runner_id", req.RunnerID).
		Str("device_id", req.DeviceID).
		Time("created_at", webhook.CreatedAt).
		Int("total_webhooks", len(s.webhooks)).
		Msg("Webhook registered")

	// Send initial available tasks
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
		Str("url", webhook.URL).
		Str("runner_id", webhook.RunnerID).
		Str("device_id", webhook.DeviceID).
		Time("created_at", webhook.CreatedAt).
		Time("unregistered_at", time.Now()).
		Int("remaining_webhooks", len(s.webhooks)).
		Msg("Webhook unregistered")

	return nil
}

func (s *WebhookService) notifyWebhooks() {
	log := gologger.WithComponent("webhook")

	select {
	case <-s.stopCh:
		log.Debug().Msg("notifyWebhooks: Ignoring webhook notification during shutdown")
		return
	default:
	}

	tasks, err := s.taskService.ListAvailableTasks(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for webhook notification")
		return
	}

	if len(tasks) == 0 {
		log.Debug().Msg("No available tasks to notify about")
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
		log.Debug().Msg("No webhooks registered, skipping notifications")
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
			log.Debug().Msg("Cancelling webhook notifications due to shutdown")
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
			Str("url", webhook.URL).
			Msg("Failed to create webhook request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", webhook.ID)

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).
			Str("webhook_id", webhook.ID).
			Str("url", webhook.URL).
			Msg("Failed to send webhook notification")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Str("webhook_id", webhook.ID).
			Str("url", webhook.URL).
			Int("status", resp.StatusCode).
			Str("response", string(body)).
			Msg("Webhook notification failed")
		return
	}

	log.Debug().
		Str("webhook_id", webhook.ID).
		Str("url", webhook.URL).
		Msg("Webhook notification sent successfully")
}

func (s *WebhookService) sendInitialNotification(webhook WebhookRegistration) {
	log := gologger.WithComponent("webhook")

	tasks, err := s.taskService.ListAvailableTasks(context.Background())
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to list tasks for initial webhook notification")
		return
	}

	payload := WSMessage{
		Type:    "available_tasks",
		Payload: tasks,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook_id", webhook.ID).
			Msg("Failed to marshal initial webhook payload")
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	s.sendWebhookNotification(client, webhook, payloadBytes)
}

func (s *WebhookService) CleanupResources() {
	log := gologger.WithComponent("webhook")

	s.webhookMutex.RLock()
	webhookCount := len(s.webhooks)
	s.webhookMutex.RUnlock()

	log.Info().
		Int("total_webhooks", webhookCount).
		Msg("Starting webhook cleanup")

	select {
	case <-s.taskUpdateCh:
	default:
	}
	close(s.taskUpdateCh)

	log.Info().
		Int("total_webhooks_cleaned", webhookCount).
		Msg("Webhook cleanup completed")
}
