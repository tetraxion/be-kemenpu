package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout bounds how long a request may run. Database calls receive the
// request context, so a query that outlives the limit is cancelled and the
// handler answers 504 instead of holding the connection open. A limit of 0
// or less disables the bound.
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
