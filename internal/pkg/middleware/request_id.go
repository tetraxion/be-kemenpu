// Package middleware holds the Gin middleware shared by all features.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	HeaderRequestID = "X-Request-ID"
	ctxKeyRequestID = "request_id"
)

// RequestID uses the client's X-Request-ID when present, or generates one.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" || len(id) > 64 {
			id = uuid.Must(uuid.NewV7()).String()
		}
		c.Set(ctxKeyRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	return c.GetString(ctxKeyRequestID)
}
