package mercado_pago

import (
	"errors"
	"net/http"

	"github.com/mercadopago/sdk-go/pkg/mperror"

	"ricardo-payments/internal/apperr"
)

// AppError traduz qualquer erro vindo do Mercado Pago (ou da nossa configuração)
// para um erro padronizado da API, com status HTTP e código estável.
func AppError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if errors.Is(err, ErrNotConfigured) {
		return &apperr.Error{Status: http.StatusServiceUnavailable, Code: "not_configured", Message: err.Error()}
	}
	if errors.Is(err, ErrTransport) {
		return &apperr.Error{
			Status:  http.StatusBadGateway,
			Code:    "outcome_unknown",
			Message: "não foi possível confirmar com o Mercado Pago; repita a chamada com a MESMA reference",
			Detail:  trim(err.Error(), 300),
		}
	}

	status, body, ok := providerStatus(err)
	if !ok {
		return &apperr.Error{
			Status:  http.StatusBadGateway,
			Code:    "provider_error",
			Message: "falha ao falar com o Mercado Pago; pode repetir com a mesma reference",
			Detail:  trim(err.Error(), 300),
		}
	}

	e := &apperr.Error{Detail: trim(body, 500)}
	switch {
	case status == 401:
		e.Status, e.Code, e.Message = http.StatusBadGateway, "provider_auth", "credencial do Mercado Pago inválida"
	case status == 403:
		e.Status, e.Code, e.Message = http.StatusBadGateway, "provider_forbidden",
			"o Mercado Pago negou a permissão (verifique se o produto está habilitado na aplicação)"
	case status == 404:
		e.Status, e.Code, e.Message = http.StatusNotFound, "not_found", "não encontrado no Mercado Pago"
	case status == 409:
		e.Status, e.Code, e.Message = http.StatusConflict, "idempotency_conflict",
			"esta reference já foi usada com dados diferentes"
	case status == 429:
		e.Status, e.Code, e.Message = http.StatusServiceUnavailable, "rate_limited", "limite de requisições do Mercado Pago; tente de novo em instantes"
	case status >= 500:
		e.Status, e.Code, e.Message = http.StatusBadGateway, "provider_error", "erro no Mercado Pago; pode repetir com a mesma reference"
	default:
		e.Status, e.Code, e.Message = http.StatusUnprocessableEntity, "provider_rejected", "o Mercado Pago recusou a operação"
	}
	return e
}

func providerStatus(err error) (int, string, bool) {
	var api *APIError
	if errors.As(err, &api) {
		return api.StatusCode, api.Body, true
	}
	var re *mperror.ResponseError
	if errors.As(err, &re) {
		return re.StatusCode, re.Message, true
	}
	return 0, "", false
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}