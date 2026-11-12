package mercado_pago

import (
	"context"
	"fmt"
	"time"

	"github.com/mercadopago/sdk-go/pkg/order"
	"github.com/mercadopago/sdk-go/pkg/requestoptions"
)

type PixCharge struct {
	OrderID      string
	Status       string
	QRCode       string // Pix copia e cola
	QRCodeBase64 string // imagem do QR
	TicketURL    string
	ExpiresAt    time.Time // zero se o MP não informar
}

// CreatePix gera a cobrança Pix pela Orders API.
// reference vira external_reference e chave de idempotência (repetir a mesma reference
// com os mesmos dados não deve criar uma segunda cobrança).
func (c *Client) CreatePix(ctx context.Context, reference string, cents int64, payerEmail string) (*PixCharge, error) {
	if payerEmail == "" {
		payerEmail = c.defaultPayerEmail
	}
	ctx = requestoptions.WithIdempotencyKey(ctx, reference)

	valor := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	res, err := c.orders.Create(ctx, order.Request{
		Type:              "online",
		ProcessingMode:    "automatic",
		TotalAmount:       valor,
		ExternalReference: reference,
		Payer:             &order.PayerRequest{Email: payerEmail},
		Transactions: &order.TransactionRequest{
			Payments: []order.PaymentRequest{{
				Amount:        valor,
				PaymentMethod: &order.PaymentMethodRequest{ID: "pix", Type: "bank_transfer"},
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mercado pago: criar pix: %w", err)
	}

	charge := &PixCharge{OrderID: res.ID, Status: normalizeStatus(res.Status)}
	if len(res.Transactions.Payments) > 0 {
		p := res.Transactions.Payments[0]
		charge.QRCode = p.PaymentMethod.QrCode
		charge.QRCodeBase64 = p.PaymentMethod.QrCodeBase64
		charge.TicketURL = p.PaymentMethod.TicketURL
		if t, err := time.Parse(time.RFC3339, p.DateOfExpiration); err == nil {
			charge.ExpiresAt = t
		}
	}
	return charge, nil
}

type OrderInfo struct {
	ID                string
	Status            string // já normalizado: approved, pending, ...
	StatusDetail      string
	ExternalReference string
	TotalAmount       string // ex.: "10.00"
	PaidAmount        string // ex.: "10.00"
}

// GetOrder consulta o status real da cobrança no Mercado Pago.
func (c *Client) GetOrder(ctx context.Context, id string) (*OrderInfo, error) {
	res, err := c.orders.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("mercado pago: consultar order %s: %w", id, err)
	}
	return &OrderInfo{
		ID:                res.ID,
		Status:            normalizeStatus(res.Status),
		StatusDetail:      res.StatusDetail,
		ExternalReference: res.ExternalReference,
		TotalAmount:       res.TotalAmount,
		PaidAmount:        res.TotalPaidAmount,
	}, nil
}

// normalizeStatus converte o status de order do MP no vocabulário do nosso sistema.
func normalizeStatus(s string) string {
	switch s {
	case "processed":
		return "approved"
	case "action_required", "created", "processing":
		return "pending"
	}
	return s
}