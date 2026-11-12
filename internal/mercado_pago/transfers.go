package mercado_pago

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const payoutURL = "https://api.mercadopago.com/v1/payouts"

var (
	// ErrNotConfigured: faltou configuração local; nada foi enviado ao Mercado Pago.
	ErrNotConfigured = errors.New("payout não configurado")
	// ErrTransport: a requisição pode ou não ter chegado; o resultado é DESCONHECIDO.
	ErrTransport = errors.New("falha de transporte")
)

// APIError: o Mercado Pago respondeu com erro HTTP (em geral 4xx = nada foi processado).
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mercado pago respondeu %d: %s", e.StatusCode, e.Body)
}

type PixTransferInput struct {
	Reference   string
	KeyType     string // CPF, CNPJ, EMAIL, PHONE... (lista exata ainda não confirmada)
	KeyValue    string
	AmountCents int64
	Description string
}

type PixTransferResult struct {
	PayoutID      string
	TransactionID string
	Status        string // "processing" quando o MP devolve "created"
}

// PayoutTxInfo é o status BRUTO de uma transação de payout, como o Mercado Pago devolve.
type PayoutTxInfo struct {
	Status       string
	StatusDetail string
}

type payoutRequest struct {
	ExternalReference string     `json:"external_reference"`
	Description       string     `json:"description,omitempty"`
	Transactions      []payoutTx `json:"transactions"`
}

type payoutTx struct {
	Description       string       `json:"description,omitempty"`
	Type              string       `json:"type"`
	Pix               payoutPix    `json:"pix"`
	Amount            payoutAmount `json:"amount"`
	ExternalReference string       `json:"external_reference"`
}

type payoutPix struct {
	Type  string `json:"type"`
	Chave string `json:"chave"`
}

type payoutAmount struct {
	Currency string  `json:"currency"`
	Value    float64 `json:"value"`
}

// payoutHeaders: headers comuns às chamadas de payout.
func (c *Client) payoutHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.payoutToken)
	req.Header.Set("Content-Type", "application/json")
	if c.payoutTest {
		req.Header.Set("X-Enforce-Signature", "false")
		req.Header.Set("X-test-token", "true")
	} else {
		req.Header.Set("X-Enforce-Signature", "true")
	}
}

// PixTransfer envia dinheiro da sua conta MP para uma chave Pix (Payouts API).
// SEM retry automático de propósito.
// BLOQUEADO até o Mercado Pago habilitar Payouts na aplicação (403 forbidden).
func (c *Client) PixTransfer(ctx context.Context, in PixTransferInput) (*PixTransferResult, error) {
	if c.payoutFake {
		return &PixTransferResult{
			PayoutID:      "FAKE-" + in.Reference,
			TransactionID: "FAKE-TX-" + in.Reference,
			Status:        "processing",
		}, nil
	}
	if !c.payoutTest && c.payoutKey == nil {
		return nil, fmt.Errorf("%w: defina MP_PAYOUT_PRIVATE_KEY_PATH (a assinatura é obrigatória em produção)", ErrNotConfigured)
	}

	body := payoutRequest{
		ExternalReference: in.Reference,
		Description:       in.Description,
		Transactions: []payoutTx{{
			Description:       in.Description,
			Type:              "pix",
			Pix:               payoutPix{Type: in.KeyType, Chave: in.KeyValue},
			Amount:            payoutAmount{Currency: "BRL", Value: float64(in.AmountCents) / 100},
			ExternalReference: in.Reference,
		}},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payoutURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	c.payoutHeaders(req)
	req.Header.Set("X-Idempotency-Key", in.Reference)
	if !c.payoutTest {
		// Produção: assina o MESMO corpo que está sendo enviado.
		req.Header.Set("X-Signature", sign(c.payoutKey, b))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransport, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}

	var out struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Transactions []struct {
			ID string `json:"id"`
		} `json:"transactions"`
	}
	_ = json.Unmarshal(raw, &out)

	res := &PixTransferResult{PayoutID: out.ID, Status: out.Status}
	if len(out.Transactions) > 0 {
		res.TransactionID = out.Transactions[0].ID
	}
	if res.Status == "" || res.Status == "created" {
		res.Status = "processing"
	}
	return res, nil
}

// GetPayoutTransaction consulta o status atual de uma transação de payout.
func (c *Client) GetPayoutTransaction(ctx context.Context, payoutID, transactionID string) (*PayoutTxInfo, error) {
	if c.payoutFake {
		return &PayoutTxInfo{Status: "success", StatusDetail: "accredited"}, nil
	}

	u := fmt.Sprintf("%s/%s/transactions/%s", payoutURL, url.PathEscape(payoutID), url.PathEscape(transactionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.payoutHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransport, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}

	var out struct {
		Status       string `json:"status"`
		StatusDetail string `json:"status_detail"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("resposta inesperada do Mercado Pago: %w", err)
	}
	return &PayoutTxInfo{Status: out.Status, StatusDetail: out.StatusDetail}, nil
}