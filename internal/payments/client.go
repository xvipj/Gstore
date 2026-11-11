package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
)

type PreferenceItem struct {
	ID        uint
	Title     string
	Quantity  int
	UnitPrice int
}

type BackURLs struct {
	Success string
	Pending string
	Failure string
}

type PreferenceInput struct {
	Items             []PreferenceItem
	ExternalReference string
	BackURLs          BackURLs
	NotificationURL   string
}

type PreferenceResult struct {
	ID        string
	InitPoint string
}

type PaymentInfo struct {
	ID                int
	Status            string
	StatusDetail      string
	ExternalReference string
	CurrencyID        string
	TransactionAmount float64
}

// Client is a thin wrapper around the official Mercado Pago Go SDK.
type Client struct {
	accessToken      string
	preferenceClient preference.Client
	paymentClient    payment.Client
}

func New(accessToken string) (*Client, error) {
	client := &Client{accessToken: strings.TrimSpace(accessToken)}
	if client.accessToken == "" {
		return client, nil
	}

	cfg, err := config.New(client.accessToken, config.WithTimeout(20*time.Second))
	if err != nil {
		return nil, fmt.Errorf("configure Mercado Pago client: %w", err)
	}

	client.preferenceClient = preference.NewClient(cfg)
	client.paymentClient = payment.NewClient(cfg)
	return client, nil
}

func (c *Client) IsConfigured() bool {
	return c.accessToken != "" && c.preferenceClient != nil && c.paymentClient != nil
}

func (c *Client) CreatePreference(ctx context.Context, input PreferenceInput) (PreferenceResult, error) {
	if !c.IsConfigured() {
		return PreferenceResult{}, errors.New("Mercado Pago is not configured")
	}

	items := make([]preference.ItemRequest, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, preference.ItemRequest{
			ID:         fmt.Sprintf("%d", item.ID),
			Title:      item.Title,
			Quantity:   item.Quantity,
			UnitPrice:  float64(item.UnitPrice),
			CurrencyID: "COP",
		})
	}

	request := preference.Request{
		Items:             items,
		ExternalReference: input.ExternalReference,
		NotificationURL:   input.NotificationURL,
		BackURLs: &preference.BackURLsRequest{
			Success: input.BackURLs.Success,
			Pending: input.BackURLs.Pending,
			Failure: input.BackURLs.Failure,
		},
	}

	response, err := c.preferenceClient.Create(ctx, request)
	if err != nil {
		return PreferenceResult{}, fmt.Errorf("create Mercado Pago preference: %w", err)
	}
	if response.InitPoint == "" {
		return PreferenceResult{}, errors.New("Mercado Pago returned an empty init_point")
	}

	return PreferenceResult{ID: response.ID, InitPoint: response.InitPoint}, nil
}

func (c *Client) GetPayment(ctx context.Context, paymentID int) (PaymentInfo, error) {
	if !c.IsConfigured() {
		return PaymentInfo{}, errors.New("Mercado Pago is not configured")
	}

	response, err := c.paymentClient.Get(ctx, paymentID)
	if err != nil {
		return PaymentInfo{}, fmt.Errorf("get Mercado Pago payment: %w", err)
	}

	return PaymentInfo{
		ID:                response.ID,
		Status:            response.Status,
		StatusDetail:      response.StatusDetail,
		ExternalReference: response.ExternalReference,
		CurrencyID:        response.CurrencyID,
		TransactionAmount: response.TransactionAmount,
	}, nil
}
