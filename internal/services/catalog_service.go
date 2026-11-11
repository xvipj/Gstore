package services

import (
	"context"
	"fmt"
	"sort"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/repository"
)

// CatalogService exposes read operations for the public storefront.
type CatalogService struct {
	caps *repository.CapRepository
}

func NewCatalogService(caps *repository.CapRepository) *CatalogService {
	return &CatalogService{caps: caps}
}

func (s *CatalogService) ListCaps(ctx context.Context, color string) ([]models.Cap, error) {
	return s.caps.ListPublished(ctx, color)
}

func (s *CatalogService) Colors(ctx context.Context) ([]string, error) {
	return s.caps.PublishedColors(ctx)
}

func (s *CatalogService) GetCap(ctx context.Context, id uint) (*models.Cap, error) {
	return s.caps.GetByID(ctx, id)
}

type CartItemInput struct {
	CapID    uint `json:"id"`
	Quantity int  `json:"quantity"`
}

type CartLine struct {
	Cap       models.Cap
	Quantity  int
	LineTotal int
}

type CartPreview struct {
	Items []CartPreviewItem `json:"items"`
	Total int               `json:"total"`
}

type CartPreviewItem struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Price     int    `json:"price"`
	Quantity  int    `json:"quantity"`
	Stock     int    `json:"stock"`
	ImagePath string `json:"image_path"`
	LineTotal int    `json:"line_total"`
}

// CartService validates cart payloads against SQLite. Prices always come from
// the database, never from localStorage.
type CartService struct {
	caps *repository.CapRepository
}

func NewCartService(caps *repository.CapRepository) *CartService {
	return &CartService{caps: caps}
}

func (s *CartService) Preview(ctx context.Context, inputs []CartItemInput) (CartPreview, error) {
	lines, err := s.GetLines(ctx, inputs)
	if err != nil {
		return CartPreview{}, err
	}

	preview := CartPreview{
		Items: make([]CartPreviewItem, 0, len(lines)),
	}
	for _, line := range lines {
		preview.Total += line.LineTotal
		preview.Items = append(preview.Items, CartPreviewItem{
			ID:        line.Cap.ID,
			Name:      line.Cap.Name,
			Color:     line.Cap.Color,
			Price:     line.Cap.Price,
			Quantity:  line.Quantity,
			Stock:     line.Cap.Stock,
			ImagePath: line.Cap.ImagePath,
			LineTotal: line.LineTotal,
		})
	}
	return preview, nil
}

func (s *CartService) GetLines(ctx context.Context, inputs []CartItemInput) ([]CartLine, error) {
	normalized, err := normalizeCartItems(inputs)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(normalized))
	for _, item := range normalized {
		ids = append(ids, item.CapID)
	}

	caps, err := s.caps.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load caps: %w", err)
	}

	byID := make(map[uint]models.Cap, len(caps))
	for _, cap := range caps {
		byID[cap.ID] = cap
	}

	lines := make([]CartLine, 0, len(normalized))
	for _, item := range normalized {
		cap, ok := byID[item.CapID]
		if !ok || !cap.Published {
			return nil, fmt.Errorf("%w: one of the selected caps is no longer available", ErrCapUnavailable)
		}
		if cap.Stock < item.Quantity {
			return nil, &StockError{CapName: cap.Name, Available: cap.Stock}
		}
		lines = append(lines, CartLine{
			Cap:       cap,
			Quantity:  item.Quantity,
			LineTotal: cap.Price * item.Quantity,
		})
	}
	return lines, nil
}

func normalizeCartItems(inputs []CartItemInput) ([]CartItemInput, error) {
	if len(inputs) == 0 {
		return nil, ErrEmptyCart
	}

	merged := make(map[uint]int)
	for _, item := range inputs {
		if item.CapID == 0 {
			return nil, fmt.Errorf("%w: invalid cap id", ErrInvalidQuantity)
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("%w: quantity must be greater than zero", ErrInvalidQuantity)
		}
		merged[item.CapID] += item.Quantity
	}

	normalized := make([]CartItemInput, 0, len(merged))
	for capID, quantity := range merged {
		normalized = append(normalized, CartItemInput{CapID: capID, Quantity: quantity})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].CapID < normalized[j].CapID
	})
	return normalized, nil
}
