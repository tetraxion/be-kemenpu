package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout cancels the request context at the limit. 0 or less disables it.
func Timeout(limit time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), limit)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
