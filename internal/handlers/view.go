package handlers

import (
	"time"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/services"
)

type BaseData struct {
	Title         string
	Page          string
	Year          int
	CSRFToken     string
	AdminUsername string
}

func baseData(title, page string) BaseData {
	return BaseData{
		Title: title,
		Page:  page,
		Year:  time.Now().Year(),
	}
}

type StorePageData struct {
	BaseData
	Caps          []models.Cap
	Colors        []string
	SelectedColor string
}

type CartPageData struct {
	BaseData
}

type ResultPageData struct {
	BaseData
	Heading string
	Message string
	Success bool
}

type ErrorPageData struct {
	BaseData
	Code    int
	Message string
}

type AdminLoginData struct {
	BaseData
	Username string
	Error    string
}

type AdminDashboardData struct {
	BaseData
	Summary services.AdminSummary
	Caps    []models.Cap
	Orders  []models.Order
}

type CapForm struct {
	Name        string
	Description string
	Color       string
	Price       string
	Stock       string
	Published   bool
	ImagePath   string
}

type AdminCapFormData struct {
	BaseData
	Form        CapForm
	FieldErrors map[string]string
	Error       string
	IsEdit      bool
	CapID       uint
	Action      string
}
