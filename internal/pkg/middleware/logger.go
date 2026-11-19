package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger records every request.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		log.LogAttrs(c.Request.Context(), level, "http request",
			slog.String("request_id", GetRequestID(c)),
			slog.String("method", c.Request.Method),
			// route is the registered pattern, empty for an unknown address;
			// path is what the client actually asked for.
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
		)
	}
}

// Recovery catches a panic and answers 500 in the standard error shape.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.ErrorContext(c.Request.Context(), "panic",
					"request_id", GetRequestID(c), "panic", rec, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":       "INTERNAL_ERROR",
						"message":    "terjadi kesalahan pada server",
						"request_id": GetRequestID(c),
					},
				})
			}
		}()
		c.Next()
	}
}
