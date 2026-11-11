package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const adminUsernameKey = "admin_username"

func RequireAdmin(sessions *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := sessions.Load(c)
		if !ok || session.AdminID == 0 {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		c.Set(SessionContextKey, session)
		c.Set(adminUsernameKey, session.Username)
		c.Next()
	}
}

// RequireCSRF checks the per-session token on admin POST forms.
func RequireCSRF(sessions *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := sessions.Load(c)
		if !ok || session.AdminID == 0 || !sessions.ValidCSRF(session, c.PostForm("csrf_token")) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func CurrentSession(c *gin.Context) (*Session, bool) {
	value, exists := c.Get(SessionContextKey)
	if !exists {
		return nil, false
	}
	session, ok := value.(*Session)
	return session, ok
}

func Username(c *gin.Context) string {
	return c.GetString(adminUsernameKey)
}
