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

		// Skip logging successful heartbeat requests
		isHeartbeat := path == "/api/runners/heartbeat"

		if !isHeartbeat {
			log := gologger.Get().With().
				Str("request_id", requestID).
				Str("method", c.Request.Method).
				Str("path", path).
				Str("remote_addr", c.Request.RemoteAddr).
				Str("hostname", hostname).
				Logger()
			log.Info().Msg("→ Request received")
		}

		c.Next()

		if !isHeartbeat || (isHeartbeat && c.Writer.Status() != 200) {
			log := gologger.Get().With().
				Str("request_id", requestID).
				Str("method", c.Request.Method).
				Str("path", path).
				Str("remote_addr", c.Request.RemoteAddr).
				Str("hostname", hostname).
				Int("status", c.Writer.Status()).
				Dur("duration", time.Since(start)).
				Int("body_size", c.Writer.Size()).
				Logger()

			if c.Writer.Status() >= 400 {
				log.Error().Msg("← Request failed")
			} else {
				log.Info().Msg("← Request completed")
			}
		}
	}
}
