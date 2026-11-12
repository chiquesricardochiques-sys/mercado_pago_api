package validate

import (
	"math"
	"net/mail"
	"regexp"
	"strings"

	"ricardo-payments/internal/apperr"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var KeyTypes = map[string]bool{"EMAIL": true, "PHONE": true, "CPF": true, "CNPJ": true, "PIX_CODE": true}

// Reference valida a reference que o app manda (vira external_reference e chave de idempotência).
func Reference(ref string) error {
	if !idRe.MatchString(ref) {
		return apperr.Validation("reference deve ter de 1 a 64 caracteres (letras, números, - e _)")
	}
	return nil
}

// ID valida ids devolvidos pelo Mercado Pago antes de usá-los em URLs.
func ID(id string) error {
	if !idRe.MatchString(id) {
		return apperr.Validation("id inválido")
	}
	return nil
}

// AmountCents converte reais em centavos, recusando valores inválidos ou com mais de 2 casas.
func AmountCents(amount, max float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, apperr.Validation("amount deve ser maior que zero")
	}
	cents := math.Round(amount * 100)
	if math.Abs(amount*100-cents) > 1e-6 {
		return 0, apperr.Validation("amount deve ter no máximo 2 casas decimais")
	}
	if max > 0 && amount > max {
		return 0, apperr.Validation("amount acima do limite configurado")
	}
	return int64(cents), nil
}

func Email(s string) error {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > 77 {
		return apperr.Validation("e-mail inválido")
	}
	return nil
}

// PixKey valida a chave Pix conforme o tipo e devolve o tipo normalizado.
func PixKey(kind, key string) (string, string, error) {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	key = strings.TrimSpace(key)

	if !KeyTypes[kind] {
		return "", "", apperr.Validation("key_type deve ser EMAIL, PHONE, CPF, CNPJ ou PIX_CODE")
	}
	if key == "" || len(key) > 140 {
		return "", "", apperr.Validation("key é obrigatório")
	}

	switch kind {
	case "CPF":
		if digits(key) != 11 {
			return "", "", apperr.Validation("CPF deve ter 11 dígitos")
		}
	case "CNPJ":
		if digits(key) != 14 {
			return "", "", apperr.Validation("CNPJ deve ter 14 dígitos")
		}
	case "EMAIL":
		if err := Email(key); err != nil {
			return "", "", apperr.Validation("chave Pix do tipo EMAIL inválida")
		}
	case "PHONE":
		if n := digits(key); n < 10 || n > 13 {
			return "", "", apperr.Validation("chave Pix do tipo PHONE inválida")
		}
	}
	return kind, key, nil
}

func digits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}