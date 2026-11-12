package mercado_pago

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// loadPrivateKey lê uma chave Ed25519 em PEM (formato PKCS#8, o mesmo do openssl genpkey).
func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler chave privada do payout: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("chave privada do payout: PEM inválido")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("chave privada do payout: %w", err)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("chave privada do payout não é Ed25519")
	}
	return priv, nil
}

// sign assina o corpo exato da requisição e devolve em Base64 (header X-Signature).
func sign(priv ed25519.PrivateKey, body []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, body))
}