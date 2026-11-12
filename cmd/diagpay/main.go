package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"ricardo-payments/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	c := &http.Client{Timeout: 20 * time.Second}
	const url = "https://api.mercadopago.com/v1/payouts"

	body := func(pixType, chave string) string {
		ref := "diag" + uuid.NewString()[:8]
		return fmt.Sprintf(`{"external_reference":%q,"description":"diag payout","transactions":[{"description":"diag","type":"pix","pix":{"type":%q,"chave":%q},"amount":{"currency":"BRL","value":10},"external_reference":%q}]}`,
			ref, pixType, chave, ref)
	}

	call := func(nome, b string) {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(b))
		req.Header.Set("Authorization", "Bearer "+cfg.MPAccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", uuid.NewString())
		req.Header.Set("X-enforce-signature", "false")
		req.Header.Set("X-test-token", "true") // SEMPRE ligado: nunca roda fora do ambiente de teste
		resp, err := c.Do(req)
		if err != nil {
			fmt.Printf("\n=== %s\nERRO DE REDE: %v\n", nome, err)
			return
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		s := string(raw)
		if len(s) > 800 {
			s = s[:800] + "...(cortado)"
		}
		fmt.Printf("\n=== %s\nHTTP %d\n%s\n", nome, resp.StatusCode, s)
	}

	call("1) chave CPF (a do exemplo da doc)", body("CPF", "111.444.777-35"))
	call("2) chave EMAIL", body("EMAIL", "destino.teste@exemplo.com"))
}