package mercado_pago

import (
	"crypto/ed25519"
	"net/http"
	"time"

	mpconfig "github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/order"

	appconfig "ricardo-payments/internal/config"
)

type Client struct {
	orders            order.Client
	http              *http.Client // usado no payout (sem retry automático)
	payoutToken       string
	payoutKey         ed25519.PrivateKey // nil se não configurada
	payoutFake        bool
	defaultPayerEmail string
	payoutTest        bool
}

func New(cfg *appconfig.Config) (*Client, error) {
	sdkCfg, err := mpconfig.New(cfg.MPAccessToken)
	if err != nil {
		return nil, err
	}

	var key ed25519.PrivateKey
	if cfg.PayoutPrivateKeyPath != "" {
		key, err = loadPrivateKey(cfg.PayoutPrivateKeyPath)
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		orders:            order.NewClient(sdkCfg),
		http:              &http.Client{Timeout: 20 * time.Second},
		payoutToken:       cfg.MPPayoutAccessToken,
		payoutKey:         key,
		payoutFake:        cfg.PayoutFake,
		defaultPayerEmail: cfg.DefaultPayerEmail,
		payoutTest:        cfg.PayoutTestMode,
	}, nil
}