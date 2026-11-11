package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	sessionCookieName = "gorras_admin"
	sessionDuration   = 8 * time.Hour
	SessionContextKey = "admin_session"
)

// Session is a signed, HttpOnly cookie payload. It contains no password data.
type Session struct {
	AdminID   uint   `json:"admin_id"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrf_token"`
	ExpiresAt int64  `json:"expires_at"`
}

type SessionManager struct {
	secret []byte
	secure bool
}

func NewSessionManager(secret string, secure bool) *SessionManager {
	return &SessionManager{secret: []byte(secret), secure: secure}
}

func (m *SessionManager) New() *Session {
	return &Session{
		CSRFToken: randomToken(32),
		ExpiresAt: time.Now().Add(sessionDuration).Unix(),
	}
}

func (m *SessionManager) Login(session *Session, adminID uint, username string) {
	session.AdminID = adminID
	session.Username = username
	session.CSRFToken = randomToken(32)
	session.ExpiresAt = time.Now().Add(sessionDuration).Unix()
}

func (m *SessionManager) Load(c *gin.Context) (*Session, bool) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || cookie == "" {
		return nil, false
	}

	parts := splitToken(cookie)
	if parts == nil {
		return nil, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, false
	}

	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, false
	}
	if session.ExpiresAt <= time.Now().Unix() {
		return nil, false
	}
	return &session, true
}

func (m *SessionManager) Save(c *gin.Context, session *Session) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(encoded))
	token := encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		Expires:  time.Unix(session.ExpiresAt, 0),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (m *SessionManager) Clear(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) ValidCSRF(session *Session, token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) == 1
}

func splitToken(token string) []string {
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			if i == 0 || i == len(token)-1 {
				return nil
			}
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}

func randomToken(size int) string {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		panic("cannot generate random token")
	}
	return hex.EncodeToString(buffer)
}
