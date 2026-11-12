package receive

import (
	"context"
	"time"

	"ricardo-payments/internal/apperr"
	"ricardo-payments/internal/mercado_pago"
	"ricardo-payments/internal/validate"
)

type Service struct {
	mp        *mercado_pago.Client
	maxAmount float64
}

func NewService(mp *mercado_pago.Client, maxAmount float64) *Service {
	return &Service{mp: mp, maxAmount: maxAmount}
}

type Input struct {
	Reference   string  `json:"reference"` // obrigatório: use o id do agendamento
	Amount      float64 `json:"amount"`    // em reais
	Description string  `json:"description"`
	PayerEmail  string  `json:"payer_email"`
}

type Charge struct {
	ID            string     `json:"id"` // salve este id no agendamento
	Reference     string     `json:"reference"`
	Status        string     `json:"status"`
	AmountCents   int64      `json:"amount_cents"`
	PixCopiaECola string     `json:"pix_copia_cola"`
	QRCodeBase64  string     `json:"qr_code_base64,omitempty"`
	TicketURL     string     `json:"ticket_url,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

func (s *Service) Create(ctx context.Context, in Input) (*Charge, error) {
	if err := validate.Reference(in.Reference); err != nil {
		return nil, err
	}
	cents, err := validate.AmountCents(in.Amount, s.maxAmount)
	if err != nil {
		return nil, err
	}
	if in.PayerEmail != "" {
		if err := validate.Email(in.PayerEmail); err != nil {
			return nil, err
		}
	}

	c, err := s.mp.CreatePix(ctx, in.Reference, cents, in.PayerEmail)
	if err != nil {
		return nil, mercado_pago.AppError(err)
	}
	if c.QRCode == "" {
		return nil, &apperr.Error{Status: 502, Code: "provider_error", Message: "o Mercado Pago não devolveu o código Pix"}
	}

	out := &Charge{
		ID: c.OrderID, Reference: in.Reference, Status: c.Status, AmountCents: cents,
		PixCopiaECola: c.QRCode, QRCodeBase64: c.QRCodeBase64, TicketURL: c.TicketURL,
	}
	if !c.ExpiresAt.IsZero() {
		exp := c.ExpiresAt
		out.ExpiresAt = &exp
	}
	return out, nil
}

type Status struct {
	ID              string `json:"id"`
	Reference       string `json:"reference"`
	Status          string `json:"status"` // pending | approved | ...
	StatusDetail    string `json:"status_detail,omitempty"`
	TotalAmount     string `json:"total_amount,omitempty"`
	TotalPaidAmount string `json:"total_paid_amount,omitempty"`
}

func (s *Service) Get(ctx context.Context, id string) (*Status, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	info, err := s.mp.GetOrder(ctx, id)
	if err != nil {
		return nil, mercado_pago.AppError(err)
	}
	return &Status{
		ID: info.ID, Reference: info.ExternalReference, Status: info.Status,
		StatusDetail: info.StatusDetail, TotalAmount: info.TotalAmount, TotalPaidAmount: info.PaidAmount,
	}, nil
}