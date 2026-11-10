package logging

import "github.com/gin-gonic/gin"

// SecurityHeaders sets conservative response headers on every response.
// The API serves no HTML, so the frame policy is simply DENY.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Render terminates TLS; the API is never served over plain HTTP
		// intentionally.
		h.Set("Strict-Transport-Security", "max-age=31536000")
		c.Next()
	}
}
