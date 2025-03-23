package services

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/theblitlabs/gologger"
)

type HeartbeatService struct {
	runnerService    *RunnerService
	scheduler        *gocron.Scheduler
	mutex            sync.Mutex
	checkInterval    time.Duration
	heartbeatTimeout time.Duration
	isRunning        bool
	stopCh           chan struct{}
}

func NewHeartbeatService(runnerService *RunnerService) *HeartbeatService {
	return &HeartbeatService{
		runnerService:    runnerService,
		checkInterval:    1 * time.Minute,
		heartbeatTimeout: 5 * time.Minute,
		stopCh:           make(chan struct{}),
	}
}

func (s *HeartbeatService) SetCheckInterval(interval time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.checkInterval = interval
}

func (s *HeartbeatService) SetHeartbeatTimeout(timeout time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.heartbeatTimeout = timeout

	s.runnerService.SetHeartbeatTimeout(timeout)
}

func (s *HeartbeatService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return nil
	}

	log := gologger.WithComponent("heartbeat_service")
	log.Info().
		Dur("check_interval", s.checkInterval).
		Dur("timeout", s.heartbeatTimeout).
		Msg("Starting heartbeat monitoring service")

	s.scheduler = gocron.NewScheduler(time.UTC)

	s.stopCh = make(chan struct{})

	job, err := s.scheduler.Every(s.checkInterval).Do(func() {
		select {
		case <-s.stopCh:
			return
		default:
			log.Debug().Msg("Running scheduled heartbeat check")
			bgCtx := context.Background()
			startTime := time.Now()

			if err := s.runnerService.UpdateOfflineRunners(bgCtx); err != nil {
				log.Error().Err(err).Msg("Error updating offline runners")
			} else {
				log.Debug().
					Dur("duration", time.Since(startTime)).
					Msg("Completed heartbeat check")
			}
		}
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to schedule heartbeat check")
		return err
	}

	s.scheduler.StartAsync()
	s.isRunning = true

	log.Info().
		Str("next_run", job.NextRun().String()).
		Msg("Heartbeat monitoring service started")

	return nil
}

func (s *HeartbeatService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return
	}

	close(s.stopCh)

	if s.scheduler != nil {
		s.scheduler.Stop()
	}

	s.isRunning = false

	log := gologger.WithComponent("heartbeat_service")
	log.Info().Msg("Heartbeat monitoring service stopped")
}

func (s *HeartbeatService) IsRunning() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.isRunning
}