package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	mpwebhook "github.com/mercadopago/sdk-go/pkg/webhook"

	"ricardo-payments/internal/mercado_pago"
	"ricardo-payments/internal/transactions"
)

type OrderGetter interface {
	GetOrder(ctx context.Context, id string) (*mercado_pago.OrderInfo, error)
}

type Handler struct {
	Secret string
	MP     OrderGetter
	Store  transactions.Store
}

type notification struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var n notification
	_ = json.Unmarshal(body, &n)

	dataID := r.URL.Query().Get("data.id")
	if dataID == "" {
		dataID = n.Data.ID
	}

	if err := mpwebhook.ValidateSignature(
		r.Header.Get("x-signature"), r.Header.Get("x-request-id"), dataID, h.Secret,
		mpwebhook.WithTolerance(5*time.Minute),
	); err != nil {
		log.Printf("webhook: assinatura inválida: %v", err)
		http.Error(w, "assinatura inválida", http.StatusUnauthorized)
		return
	}

	topic := r.URL.Query().Get("type")
	if topic == "" {
		topic = n.Type
	}
	if topic != "order" {
		log.Printf("webhook: tópico %q ignorado por enquanto: %s", topic, string(body))
		w.WriteHeader(http.StatusOK)
		return
	}
	if dataID == "" {
		http.Error(w, "data.id ausente", http.StatusBadRequest)
		return
	}

	// Não confia no corpo: consulta o status real no Mercado Pago.
	info, err := h.MP.GetOrder(r.Context(), dataID)
	if err != nil {
		log.Printf("webhook: erro consultando order %s: %v", dataID, err)
		http.Error(w, "erro interno", http.StatusInternalServerError) // MP tenta de novo
		return
	}

	if _, err := h.Store.Update(info.ExternalReference, func(t *transactions.Transaction) {
		t.Status = info.Status
		t.StatusDetail = info.StatusDetail
	}); err != nil {
		log.Printf("webhook: reference %q não encontrada (order %s)", info.ExternalReference, dataID)
	}

	log.Printf("webhook: order %s (%s) -> %s", dataID, info.ExternalReference, info.Status)
	w.WriteHeader(http.StatusOK)
}