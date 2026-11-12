package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	APIKey               string
	MPAccessToken        string // cobranças (receber)
	MPPayoutAccessToken  string // payouts (pagar); se vazio, usa MPAccessToken
	PayoutPrivateKeyPath string // PEM Ed25519; obrigatório só em produção
	PayoutFake           bool   // simula o payout localmente (só em modo teste)
	PayoutTestMode       bool
	PayRequiresOrder     bool // exige order_id (cobrança paga) em todo /pay
	DefaultPayerEmail    string
	MaxPayAmount         float64
	MaxReceiveAmount     float64
}

func Load() (*Config, error) {
	loadDotEnv()

	maxPay, err := getFloat("MAX_PAY_AMOUNT", 50)
	if err != nil {
		return nil, err
	}
	maxReceive, err := getFloat("MAX_RECEIVE_AMOUNT", 5000)
	if err != nil {
		return nil, err
	}

	c := &Config{
		Port:                 get("PORT", "8080"),
		APIKey:               os.Getenv("API_KEY"),
		MPAccessToken:        os.Getenv("MP_ACCESS_TOKEN"),
		PayoutPrivateKeyPath: os.Getenv("MP_PAYOUT_PRIVATE_KEY_PATH"),
		PayoutFake:           get("PAYOUT_FAKE", "false") == "true",
		PayoutTestMode:       get("PAYOUT_TEST_MODE", "true") == "true",
		PayRequiresOrder:     get("PAY_REQUIRES_ORDER", "false") == "true",
		DefaultPayerEmail:    get("DEFAULT_PAYER_EMAIL", "pagador@exemplo.com"),
		MaxPayAmount:         maxPay,
		MaxReceiveAmount:     maxReceive,
	}
	c.MPPayoutAccessToken = get("MP_PAYOUT_ACCESS_TOKEN", c.MPAccessToken)

	if c.APIKey == "" || c.MPAccessToken == "" {
		return nil, fmt.Errorf("API_KEY e MP_ACCESS_TOKEN são obrigatórios")
	}
	if c.PayoutFake && !c.PayoutTestMode {
		return nil, fmt.Errorf("PAYOUT_FAKE só pode ficar ligado com PAYOUT_TEST_MODE=true")
	}
	if !c.PayoutTestMode && len(c.APIKey) < 24 {
		return nil, fmt.Errorf("em produção a API_KEY precisa ter pelo menos 24 caracteres")
	}
	return c, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getFloat(k string, def float64) (float64, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("%s inválido: %q", k, v)
	}
	return f, nil
}