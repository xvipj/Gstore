package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tienda-gorras/internal/middleware"
	"tienda-gorras/internal/models"
	"tienda-gorras/internal/services"
)

const maxImageBytes = 2 * 1024 * 1024

type AdminHandler struct {
	renderer *Renderer
	admin    *services.AdminService
	sessions *middleware.SessionManager
}

func NewAdminHandler(renderer *Renderer, admin *services.AdminService, sessions *middleware.SessionManager) *AdminHandler {
	return &AdminHandler{renderer: renderer, admin: admin, sessions: sessions}
}

func (h *AdminHandler) LoginPage(c *gin.Context) {
	session, ok := h.sessions.Load(c)
	if ok && session.AdminID != 0 {
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	if !ok {
		session = h.sessions.New()
		_ = h.sessions.Save(c, session)
	}

	data := AdminLoginData{BaseData: h.baseData(c, session, "Acceso administrador", "admin-login")}
	if err := h.renderer.RenderLogin(c, http.StatusOK, data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *AdminHandler) Login(c *gin.Context) {
	session, ok := h.sessions.Load(c)
	if !ok {
		session = h.sessions.New()
		_ = h.sessions.Save(c, session)
		h.renderLoginError(c, session, http.StatusBadRequest, "La sesión expiró. Intenta de nuevo.")
		return
	}
	if !h.sessions.ValidCSRF(session, c.PostForm("csrf_token")) {
		h.renderLoginError(c, session, http.StatusForbidden, "La solicitud no es válida. Intenta de nuevo.")
		return
	}

	username := c.PostForm("username")
	admin, err := h.admin.Authenticate(c.Request.Context(), username, c.PostForm("password"))
	if err != nil {
		// Fixed delay instead of an in-memory attempt counter, to respect the
		// project rule against caches and brute-force protection in RAM.
		time.Sleep(1200 * time.Millisecond)
		h.renderLoginError(c, session, http.StatusUnauthorized, "Usuario o contraseña incorrectos.")
		return
	}

	h.sessions.Login(session, admin.ID, admin.Username)
	if err := h.sessions.Save(c, session); err != nil {
		h.renderer.internalError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (h *AdminHandler) Logout(c *gin.Context) {
	h.sessions.Clear(c)
	c.Redirect(http.StatusSeeOther, "/admin/login")
}

func (h *AdminHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	summary, err := h.admin.Summary(ctx)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}
	caps, err := h.admin.ListCaps(ctx)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}
	orders, err := h.admin.ListOrders(ctx)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}

	data := AdminDashboardData{
		BaseData: h.base(c, "Panel", "admin"),
		Summary:  summary,
		Caps:     caps,
		Orders:   orders,
	}
	if err := h.renderer.RenderAdmin(c, http.StatusOK, "admin_dashboard.html", data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *AdminHandler) NewCapForm(c *gin.Context) {
	data := AdminCapFormData{
		BaseData: h.base(c, "Nueva gorra", "admin-cap-form"),
		Form:     CapForm{Published: true},
		IsEdit:   false,
		Action:   "/admin/caps",
	}
	h.renderCapForm(c, http.StatusOK, data)
}

func (h *AdminHandler) CreateCap(c *gin.Context) {
	form, input, fieldErrors, err := h.parseCapForm(c, true)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}

	if len(fieldErrors) == 0 {
		if _, err := h.admin.CreateCap(c.Request.Context(), input); err != nil {
			if h.applyServiceError(c, err, fieldErrors, &form) {
				h.renderCapForm(c, http.StatusUnprocessableEntity, AdminCapFormData{
					BaseData:    h.base(c, "Nueva gorra", "admin-cap-form"),
					Form:        form,
					FieldErrors: fieldErrors,
					IsEdit:      false,
					Action:      "/admin/caps",
				})
				return
			}
			h.renderer.internalError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	h.renderCapForm(c, http.StatusUnprocessableEntity, AdminCapFormData{
		BaseData:    h.base(c, "Nueva gorra", "admin-cap-form"),
		Form:        form,
		FieldErrors: fieldErrors,
		IsEdit:      false,
		Action:      "/admin/caps",
	})
}

func (h *AdminHandler) EditCapForm(c *gin.Context) {
	id, ok := capID(c)
	if !ok {
		h.renderer.RenderErrorPage(c, http.StatusNotFound)
		return
	}
	cap, err := h.admin.GetCap(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrCapNotFound) {
			h.renderer.RenderErrorPage(c, http.StatusNotFound)
			return
		}
		h.renderer.internalError(c, err)
		return
	}

	data := AdminCapFormData{
		BaseData: h.base(c, "Editar gorra", "admin-cap-form"),
		Form:     capFormFromModel(cap),
		IsEdit:   true,
		CapID:    cap.ID,
		Action:   fmt.Sprintf("/admin/caps/%d", cap.ID),
	}
	h.renderCapForm(c, http.StatusOK, data)
}

func (h *AdminHandler) UpdateCap(c *gin.Context) {
	id, ok := capID(c)
	if !ok {
		h.renderer.RenderErrorPage(c, http.StatusNotFound)
		return
	}

	form, input, fieldErrors, err := h.parseCapForm(c, false)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}

	if len(fieldErrors) == 0 {
		if _, err := h.admin.UpdateCap(c.Request.Context(), id, input); err != nil {
			if errors.Is(err, services.ErrCapNotFound) {
				h.renderer.RenderErrorPage(c, http.StatusNotFound)
				return
			}
			if h.applyServiceError(c, err, fieldErrors, &form) {
				h.renderCapForm(c, http.StatusUnprocessableEntity, AdminCapFormData{
					BaseData:    h.base(c, "Editar gorra", "admin-cap-form"),
					Form:        form,
					FieldErrors: fieldErrors,
					IsEdit:      true,
					CapID:       id,
					Action:      fmt.Sprintf("/admin/caps/%d", id),
				})
				return
			}
			h.renderer.internalError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	h.renderCapForm(c, http.StatusUnprocessableEntity, AdminCapFormData{
		BaseData:    h.base(c, "Editar gorra", "admin-cap-form"),
		Form:        form,
		FieldErrors: fieldErrors,
		IsEdit:      true,
		CapID:       id,
		Action:      fmt.Sprintf("/admin/caps/%d", id),
	})
}

func (h *AdminHandler) DeleteCap(c *gin.Context) {
	id, ok := capID(c)
	if !ok {
		h.renderer.RenderErrorPage(c, http.StatusNotFound)
		return
	}
	if err := h.admin.DeleteCap(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrCapNotFound) {
			h.renderer.RenderErrorPage(c, http.StatusNotFound)
			return
		}
		h.renderer.internalError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (h *AdminHandler) parseCapForm(c *gin.Context, requireImage bool) (CapForm, services.CapInput, map[string]string, error) {
	fieldErrors := make(map[string]string)
	form := CapForm{
		Name:        strings.TrimSpace(c.PostForm("name")),
		Description: strings.TrimSpace(c.PostForm("description")),
		Color:       strings.TrimSpace(c.PostForm("color")),
		Price:       strings.TrimSpace(c.PostForm("price")),
		Stock:       strings.TrimSpace(c.PostForm("stock")),
		Published:   c.PostForm("published") == "true",
	}

	price, err := parsePrice(form.Price)
	if err != nil {
		fieldErrors["price"] = "Ingresa un precio válido en pesos colombianos."
	}
	stock, err := parseStock(form.Stock)
	if err != nil {
		fieldErrors["stock"] = "Ingresa un stock válido."
	}

	input := services.CapInput{
		Name:        form.Name,
		Description: form.Description,
		Color:       form.Color,
		Price:       price,
		Stock:       stock,
		Published:   form.Published,
	}

	if len(fieldErrors) == 0 {
		image, imageErr := readCapImage(c)
		if imageErr != nil {
			fieldErrors["image"] = imageErr.Error()
		} else {
			input.Image = image
		}
	}

	// Merge business validation even when the upload itself failed, so every
	// invalid field is shown at once.
	if err := services.ValidateCapInput(input); err != nil {
		var validationErr *services.ValidationError
		if errors.As(err, &validationErr) {
			for field, message := range validationErr.Fields {
				fieldErrors[field] = message
			}
		}
	}
	if requireImage && input.Image == nil && fieldErrors["image"] == "" {
		fieldErrors["image"] = "La foto es obligatoria."
	}

	return form, input, fieldErrors, nil
}

func (h *AdminHandler) applyServiceError(_ *gin.Context, err error, fieldErrors map[string]string, form *CapForm) bool {
	var validationErr *services.ValidationError
	if errors.As(err, &validationErr) {
		for field, message := range validationErr.Fields {
			fieldErrors[field] = message
		}
		return true
	}
	return false
}

func (h *AdminHandler) renderCapForm(c *gin.Context, status int, data AdminCapFormData) {
	if err := h.renderer.RenderAdmin(c, status, "admin_cap_form.html", data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *AdminHandler) renderLoginError(c *gin.Context, session *middleware.Session, status int, message string) {
	data := AdminLoginData{
		BaseData: h.baseData(c, session, "Acceso administrador", "admin-login"),
		Error:    message,
	}
	if err := h.renderer.RenderLogin(c, status, data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *AdminHandler) base(c *gin.Context, title, page string) BaseData {
	session, _ := middleware.CurrentSession(c)
	return h.baseData(c, session, title, page)
}

func (h *AdminHandler) baseData(c *gin.Context, session *middleware.Session, title, page string) BaseData {
	data := baseData(title, page)
	data.AdminUsername = middleware.Username(c)
	if session != nil {
		data.CSRFToken = session.CSRFToken
	}
	return data
}

func capID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return uint(value), true
}

func capFormFromModel(cap *models.Cap) CapForm {
	return CapForm{
		Name:        cap.Name,
		Description: cap.Description,
		Color:       cap.Color,
		Price:       strconv.Itoa(cap.Price),
		Stock:       strconv.Itoa(cap.Stock),
		Published:   cap.Published,
		ImagePath:   cap.ImagePath,
	}
}

func parsePrice(value string) (int, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.NewReplacer("$", "", "COP", "", ".", "", " ", "").Replace(cleaned)
	return strconv.Atoi(cleaned)
}

func parseStock(value string) (int, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return 0, nil
	}
	return strconv.Atoi(cleaned)
}

func readCapImage(c *gin.Context) (*services.ImageUpload, error) {
	header, err := c.FormFile("image")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) || errors.Is(err, http.ErrNotMultipart) {
			return nil, nil
		}
		return nil, errors.New("No se pudo leer la foto. Revisa que no supere 2 MB.")
	}
	if header.Size > maxImageBytes {
		return nil, errors.New("La foto no puede superar 2 MB.")
	}

	file, err := header.Open()
	if err != nil {
		return nil, errors.New("No se pudo abrir la foto.")
	}
	defer file.Close()

	data, err := readLimited(file, maxImageBytes)
	if err != nil {
		return nil, errors.New("La foto no puede superar 2 MB.")
	}

	return &services.ImageUpload{Filename: header.Filename, Data: data}, nil
}
