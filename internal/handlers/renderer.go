package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Renderer parses templates on every request. This intentionally avoids any
// template cache, as required by the project.
type Renderer struct {
	templatesDir string
}

func NewRenderer(templatesDir string) *Renderer {
	return &Renderer{templatesDir: templatesDir}
}

func (r *Renderer) Render(c *gin.Context, status int, layoutFile, pageFile string, data any) error {
	layoutPath := filepath.Join(r.templatesDir, layoutFile)
	pagePath := filepath.Join(r.templatesDir, pageFile)

	tmpl, err := template.New(filepath.Base(layoutPath)).
		Funcs(template.FuncMap{
			"formatCOP":   formatCOP,
			"formatDate":  formatDate,
			"statusLabel": statusLabel,
			"truncate":    truncate,
		}).
		ParseFiles(layoutPath, pagePath)
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	var output bytes.Buffer
	layoutName := filepath.Base(layoutFile)
	if err := tmpl.ExecuteTemplate(&output, layoutName, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	c.Data(status, "text/html; charset=utf-8", output.Bytes())
	return nil
}

func (r *Renderer) RenderPublic(c *gin.Context, status int, pageFile string, data any) error {
	return r.Render(c, status, "layout.html", pageFile, data)
}

func (r *Renderer) RenderAdmin(c *gin.Context, status int, pageFile string, data any) error {
	return r.Render(c, status, "admin_layout.html", pageFile, data)
}

func (r *Renderer) RenderLogin(c *gin.Context, status int, data any) error {
	return r.Render(c, status, "login_layout.html", "admin_login.html", data)
}

func formatCOP(value any) string {
	var amount int64
	switch typed := value.(type) {
	case int:
		amount = int64(typed)
	case int64:
		amount = typed
	default:
		amount = 0
	}

	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	raw := strconv.FormatInt(amount, 10)
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return "$" + sign + strings.Join(parts, ".") + " COP"
}

func formatDate(value time.Time) string {
	return value.Format("02/01/2006 15:04")
}

func statusLabel(status string) string {
	switch status {
	case "pending":
		return "Pendiente"
	case "paid":
		return "Pagada"
	case "failed":
		return "Fallida"
	case "cancelled":
		return "Cancelada"
	default:
		return status
	}
}

func truncate(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}

func (r *Renderer) internalError(c *gin.Context, err error) {
	c.Error(err) //nolint:errcheck // Gin only stores the error for logging.
	_ = r.RenderPublic(c, http.StatusInternalServerError, "error.html", ErrorPageData{
		BaseData: baseData("Error", "error"),
		Code:     http.StatusInternalServerError,
		Message:  "Ocurrió un error inesperado.",
	})
}

// RenderErrorPage is used by the recovery middleware.
func (r *Renderer) RenderErrorPage(c *gin.Context, status int) {
	message := "Ocurrió un error inesperado."
	if status == http.StatusNotFound {
		message = "La página que buscas no existe."
	}
	if err := r.RenderPublic(c, status, "error.html", ErrorPageData{
		BaseData: baseData("Error", "error"),
		Code:     status,
		Message:  message,
	}); err != nil {
		c.String(status, "Error")
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, nil
}
