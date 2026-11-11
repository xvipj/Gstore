package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/repository"
)

const maxImageBytes = 2 * 1024 * 1024

type ImageUpload struct {
	Filename string
	Data     []byte
}

type CapInput struct {
	Name        string
	Description string
	Color       string
	Price       int
	Stock       int
	Published   bool
	Image       *ImageUpload
}

type ColorSummary struct {
	Color string
	Stock int
	Sold  int
}

type AdminSummary struct {
	PublishedCaps int64
	StockUnits    int64
	SoldUnits     int64
	Revenue       int64
	PaidOrders    int64
	Colors        []ColorSummary
}

type AdminService struct {
	admins    *repository.AdminRepository
	caps      *repository.CapRepository
	orders    *repository.OrderRepository
	uploadDir string
}

func NewAdminService(
	admins *repository.AdminRepository,
	caps *repository.CapRepository,
	orders *repository.OrderRepository,
	uploadDir string,
) *AdminService {
	return &AdminService{admins: admins, caps: caps, orders: orders, uploadDir: uploadDir}
}

func (s *AdminService) Authenticate(ctx context.Context, username, password string) (*models.AdminUser, error) {
	admin, err := s.admins.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("load admin: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return admin, nil
}

func (s *AdminService) Summary(ctx context.Context) (AdminSummary, error) {
	catalog, err := s.caps.CatalogStats(ctx)
	if err != nil {
		return AdminSummary{}, err
	}
	revenue, err := s.orders.RevenueStats(ctx)
	if err != nil {
		return AdminSummary{}, err
	}
	colors, err := s.caps.StatsByColor(ctx)
	if err != nil {
		return AdminSummary{}, err
	}

	summary := AdminSummary{
		PublishedCaps: catalog.PublishedCaps,
		StockUnits:    catalog.StockUnits,
		SoldUnits:     catalog.SoldUnits,
		Revenue:       revenue.Revenue,
		PaidOrders:    revenue.PaidOrders,
		Colors:        make([]ColorSummary, 0, len(colors)),
	}
	for _, color := range colors {
		summary.Colors = append(summary.Colors, ColorSummary{
			Color: color.Color,
			Stock: color.Stock,
			Sold:  color.Sold,
		})
	}
	return summary, nil
}

func (s *AdminService) ListCaps(ctx context.Context) ([]models.Cap, error) {
	return s.caps.ListAll(ctx)
}

func (s *AdminService) GetCap(ctx context.Context, id uint) (*models.Cap, error) {
	cap, err := s.caps.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCapNotFound
		}
		return nil, err
	}
	return cap, nil
}

func (s *AdminService) ListOrders(ctx context.Context) ([]models.Order, error) {
	return s.orders.ListWithItems(ctx)
}

func (s *AdminService) CreateCap(ctx context.Context, input CapInput) (*models.Cap, error) {
	if err := ValidateCapInput(input); err != nil {
		return nil, err
	}
	if input.Image == nil {
		return nil, &ValidationError{Fields: map[string]string{"image": "La foto es obligatoria."}}
	}

	imagePath, err := s.saveImage(*input.Image)
	if err != nil {
		return nil, err
	}

	cap := &models.Cap{
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		Color:       strings.TrimSpace(input.Color),
		Price:       input.Price,
		Stock:       input.Stock,
		ImagePath:   imagePath,
		Published:   input.Published,
	}
	if err := s.caps.Create(ctx, cap); err != nil {
		return nil, fmt.Errorf("create cap: %w", err)
	}
	return cap, nil
}

func (s *AdminService) UpdateCap(ctx context.Context, id uint, input CapInput) (*models.Cap, error) {
	cap, err := s.GetCap(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ValidateCapInput(input); err != nil {
		return nil, err
	}

	cap.Name = strings.TrimSpace(input.Name)
	cap.Description = strings.TrimSpace(input.Description)
	cap.Color = strings.TrimSpace(input.Color)
	cap.Price = input.Price
	cap.Stock = input.Stock
	cap.Published = input.Published

	if input.Image != nil {
		imagePath, err := s.saveImage(*input.Image)
		if err != nil {
			return nil, err
		}
		cap.ImagePath = imagePath
	}

	if err := s.caps.Update(ctx, cap); err != nil {
		return nil, fmt.Errorf("update cap: %w", err)
	}
	return cap, nil
}

func (s *AdminService) DeleteCap(ctx context.Context, id uint) error {
	if _, err := s.GetCap(ctx, id); err != nil {
		return err
	}
	if err := s.caps.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete cap: %w", err)
	}
	return nil
}

// ValidateCapInput exposes field validation so the admin form can display all
// errors before attempting any file write.
func ValidateCapInput(input CapInput) error {
	fields := make(map[string]string)

	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "El nombre es obligatorio."
	}
	if strings.TrimSpace(input.Color) == "" {
		fields["color"] = "El color es obligatorio."
	}
	if input.Price <= 0 {
		fields["price"] = "El precio debe ser mayor que cero."
	}
	if input.Stock < 0 {
		fields["stock"] = "El stock no puede ser negativo."
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func (s *AdminService) saveImage(upload ImageUpload) (string, error) {
	if len(upload.Data) == 0 {
		return "", &ValidationError{Fields: map[string]string{"image": "La foto está vacía."}}
	}
	if len(upload.Data) > maxImageBytes {
		return "", &ValidationError{Fields: map[string]string{"image": "La foto no puede superar 2 MB."}}
	}

	originalExt := strings.ToLower(filepath.Ext(upload.Filename))
	allowedExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowedExt[originalExt] {
		return "", &ValidationError{Fields: map[string]string{"image": "Formato no permitido. Usa JPG, PNG o WebP."}}
	}

	detectedType := http.DetectContentType(upload.Data)
	extByType := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}
	savedExt, allowedType := extByType[detectedType]
	if !allowedType {
		return "", &ValidationError{Fields: map[string]string{"image": "El archivo no es una imagen JPG, PNG o WebP válida."}}
	}

	randomName := make([]byte, 16)
	if _, err := rand.Read(randomName); err != nil {
		return "", fmt.Errorf("generate image name: %w", err)
	}

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}

	filename := hex.EncodeToString(randomName) + savedExt
	destination := filepath.Join(s.uploadDir, filename)
	if err := os.WriteFile(destination, upload.Data, 0o644); err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}

	return "/static/uploads/" + filename, nil
}
