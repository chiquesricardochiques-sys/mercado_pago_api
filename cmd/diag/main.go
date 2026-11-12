package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricardo-payments/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("token carregado do .env: tamanho=%d inicio=%.8s\n", len(cfg.MPAccessToken), cfg.MPAccessToken)

	c := &http.Client{Timeout: 20 * time.Second}

	call := func(nome, method, url, body string, max int) {
		var rd io.Reader
		if body != "" {
			rd = bytes.NewBufferString(body)
		}
		req, _ := http.NewRequest(method, url, rd)
		req.Header.Set("Authorization", "Bearer "+cfg.MPAccessToken)
		req.Header.Set("Content-Type", "application/json")
		if method == http.MethodPost {
			req.Header.Set("X-Idempotency-Key", uuid.NewString())
		}
		resp, err := c.Do(req)
		if err != nil {
			fmt.Printf("\n=== %s\nERRO DE REDE: %v\n", nome, err)
			return
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		s := string(raw)
		temQR := strings.Contains(s, "qr_code")
		if len(s) > max {
			s = s[:max] + "...(cortado)"
		}
		fmt.Printf("\n=== %s\nHTTP %d | contem qr_code: %v\n%s\n", nome, resp.StatusCode, temQR, s)
	}

	pix := func(email, notif string) string {
		n := ""
		if notif != "" {
			n = fmt.Sprintf(`,"notification_url":%q`, notif)
		}
		return fmt.Sprintf(`{"transaction_amount":10,"payment_method_id":"pix","description":"diag","payer":{"email":%q}%s}`, email, n)
	}

	order := fmt.Sprintf(`{"type":"online","processing_mode":"automatic","total_amount":"10.00","external_reference":"diag-%s","payer":{"email":%q},"transactions":{"payments":[{"amount":"10.00","payment_method":{"id":"pix","type":"bank_transfer"}}]}}`,
		uuid.NewString()[:8], cfg.DefaultPayerEmail)

	call("0) quem é o dono do token (GET /users/me)", "GET", "https://api.mercadopago.com/users/me", "", 200)
	call("A) Pix via /v1/payments, sem notification_url", "POST", "https://api.mercadopago.com/v1/payments", pix(cfg.DefaultPayerEmail, ""), 500)
	call("B) Pix via /v1/payments, com a notification_url do seu .env", "POST", "https://api.mercadopago.com/v1/payments", pix(cfg.DefaultPayerEmail, cfg.NotificationURL()), 500)
	call("C) Pix via /v1/payments, comprador diferente", "POST", "https://api.mercadopago.com/v1/payments", pix("comprador@exemplo.com", ""), 500)
	call("D) Pix via /v1/orders (a Orders API do seu main.go antigo)", "POST", "https://api.mercadopago.com/v1/orders", order, 500)
}