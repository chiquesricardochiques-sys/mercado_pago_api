package pay

import (
	"context"
	"math"
	"strconv"
	"strings"

	"ricardo-payments/internal/apperr"
	"ricardo-payments/internal/mercado_pago"
	"ricardo-payments/internal/validate"
)

type Service struct {
	mp           *mercado_pago.Client
	maxAmount    float64
	requireOrder bool
}

func NewService(mp *mercado_pago.Client, maxAmount float64, requireOrder bool) *Service {
	return &Service{mp: mp, maxAmount: maxAmount, requireOrder: requireOrder}
}

type Input struct {
	Reference   string  `json:"reference"` // OBRIGATÓRIO e FIXO por repasse: é a trava contra pagar duas vezes
	KeyType     string  `json:"key_type"`  // EMAIL | PHONE | CPF | CNPJ | PIX_CODE
	Key         string  `json:"key"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	OrderID     string  `json:"order_id"` // opcional: id da cobrança (do /receive) que originou este repasse
}

type Result struct {
	ID        string `json:"id"` // formato payout:transação; salve no agendamento
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

func (s *Service) Pay(ctx context.Context, in Input) (*Result, error) {
	if err := validate.Reference(in.Reference); err != nil {
		return nil, err
	}
	kind, key, err := validate.PixKey(in.KeyType, in.Key)
	if err != nil {
		return nil, err
	}
	cents, err := validate.AmountCents(in.Amount, s.maxAmount)
	if err != nil {
		return nil, err
	}

	if s.requireOrder && in.OrderID == "" {
		return nil, apperr.Validation("order_id é obrigatório")
	}
	if in.OrderID != "" {
		if err := s.checkOrder(ctx, in.OrderID, cents); err != nil {
			return nil, err
		}
	}

	res, err := s.mp.PixTransfer(ctx, mercado_pago.PixTransferInput{
		Reference: in.Reference, KeyType: kind, KeyValue: key,
		AmountCents: cents, Description: in.Description,
	})
	if err != nil {
		return nil, mercado_pago.AppError(err)
	}

	return &Result{
		ID:        res.PayoutID + ":" + res.TransactionID,
		Reference: in.Reference,
		Status:    res.Status,
	}, nil
}

// checkOrder só libera o repasse se a cobrança estiver PAGA e o valor não passar do que foi recebido.
// Limite: sem banco, esta API não impede dois repasses diferentes contra a mesma cobrança.
// Quem controla isso é o app (um repasse por agendamento).
func (s *Service) checkOrder(ctx context.Context, orderID string, payCents int64) error {
	if err := validate.ID(orderID); err != nil {
		return err
	}
	info, err := s.mp.GetOrder(ctx, orderID)
	if err != nil {
		return mercado_pago.AppError(err)
	}
	if info.Status != "approved" {
		return apperr.Conflict("order_not_paid", "a cobrança ainda não foi paga (status: "+info.Status+")")
	}
	paid, err := parseCents(info.PaidAmount)
	if err != nil {
		return &apperr.Error{Status: 502, Code: "provider_error", Message: "não foi possível ler o valor pago da cobrança"}
	}
	if payCents > paid {
		return apperr.Conflict("amount_exceeds_paid", "o valor do repasse é maior que o valor pago na cobrança")
	}
	return nil
}

type StatusResult struct {
	ID           string `json:"id"`
	Status       string `json:"status"` // processing | approved | failed | refunded | canceled
	StatusDetail string `json:"status_detail,omitempty"`
}

func (s *Service) Get(ctx context.Context, id string) (*StatusResult, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || validate.ID(parts[0]) != nil || validate.ID(parts[1]) != nil {
		return nil, apperr.Validation("id inválido (formato payout:transação)")
	}
	info, err := s.mp.GetPayoutTransaction(ctx, parts[0], parts[1])
	if err != nil {
		return nil, mercado_pago.AppError(err)
	}
	return &StatusResult{
		ID:           id,
		Status:       mapPayoutStatus(info.Status, info.StatusDetail),
		StatusDetail: info.StatusDetail,
	}, nil
}

func parseCents(s string) (int64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}

// mapPayoutStatus traduz o status de transação do Payouts para o vocabulário do nosso sistema.
// Em "failed", a doc manda executar de novo: use uma reference NOVA.
func mapPayoutStatus(status, detail string) string {
	switch status {
	case "success":
		if detail == "in_progress" {
			return "processing"
		}
		return "approved"
	case "processed":
		return "approved"
	case "approved":
		if detail == "partially_refunded" {
			return "approved"
		}
		return "processing" // status intermediário
	case "created", "transaction_in_process":
		return "processing"
	case "error", "rejected":
		return "failed"
	case "refunded":
		return "refunded"
	case "canceled":
		return "canceled"
	}
	return "processing"
}