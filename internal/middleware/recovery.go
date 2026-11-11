package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

type ErrorRenderer func(c *gin.Context, status int)

// Recovery logs panics and delegates to the HTML error renderer.
func Recovery(render ErrorRenderer) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("[panic] %v\n%s", recovered, debug.Stack())
		if c.Writer.Written() {
			c.Abort()
			return
		}
		render(c, http.StatusInternalServerError)
		c.Abort()
	})
}

// NoStore disables browser and intermediary caching for every response.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

// LimitBody protects form parsing from oversized requests.
func LimitBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
