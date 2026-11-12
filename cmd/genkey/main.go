package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Fatal(err)
	}

	write("mpprivate.pem", "PRIVATE KEY", privDER, 0o600)
	write("mppublic.pem", "PUBLIC KEY", pubDER, 0o644)
	fmt.Println("Pronto: mpprivate.pem (SECRETA, nunca compartilhe nem suba no git) e mppublic.pem (esta vai pro Mercado Pago).")
}

func write(name, kind string, der []byte, perm os.FileMode) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) // não sobrescreve
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: kind, Bytes: der}); err != nil {
		log.Fatal(err)
	}
}