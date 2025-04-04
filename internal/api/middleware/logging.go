package middleware

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
)

func Logging() gin.HandlerFunc {
	hostname, err := os.Hostname()
	if err != nil {
		log := gologger.Get()
		log.Error().Err(err).Msg("Failed to get hostname")
		hostname = "unknown"
	}
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		log := gologger.Get().With().
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("remote_addr", c.Request.RemoteAddr).
			Str("hostname", hostname).
			Logger()

		log.Info().Msg("→ Request received")

		c.Next()

		// Skip successful heartbeat logs to reduce noise
		isHeartbeat := c.Request.URL.Path == "/runners/heartbeat"
		if !isHeartbeat || (isHeartbeat && c.Writer.Status() != 200) {
			respLog := log.With().
				Int("status", c.Writer.Status()).
				Dur("duration", time.Since(start)).
				Int("body_size", c.Writer.Size()).
				Logger()

			if c.Writer.Status() >= 400 {
				respLog.Error().Msg("← Request failed")
			} else {
				respLog.Info().Msg("← Request completed")
			}
		}
	}
}
