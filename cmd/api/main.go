package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ricardo-payments/internal/config"
	httpapi "ricardo-payments/internal/http"
	"ricardo-payments/internal/mercado_pago"
	"ricardo-payments/internal/modules/pay"
	"ricardo-payments/internal/modules/receive"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("payout: fake=%v modo_teste=%v exige_order=%v chave_privada=%v",
		cfg.PayoutFake, cfg.PayoutTestMode, cfg.PayRequiresOrder, cfg.PayoutPrivateKeyPath != "")
	if cfg.PayoutFake {
		log.Println("ATENÇÃO: PAYOUT_FAKE ligado, os repasses são SIMULADOS (nenhum dinheiro se move)")
	}

	mp, err := mercado_pago.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Deps{
			APIKey:  cfg.APIKey,
			Receive: receive.NewService(mp, cfg.MaxReceiveAmount),
			Pay:     pay.NewService(mp, cfg.MaxPayAmount, cfg.PayRequiresOrder),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("RICARDO-PAYMENTS na porta :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Desligamento limpo (Ctrl+C ou SIGTERM da hospedagem): termina as requisições em andamento.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("erro ao desligar: %v", err)
	}
	log.Println("servidor encerrado")
}