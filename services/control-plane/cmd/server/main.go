package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/linux-server-manager/backend/internal/api"
	"github.com/linux-server-manager/backend/internal/config"
	"github.com/linux-server-manager/backend/internal/database"
)

func main() {
	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("[LSM Server] Erro de validação de configuração: %v", err)
	}

	log.Printf("[LSM Server] Inicializando Linux Server Manager Backend (Ambiente: %s)...", cfg.Environment)

	// Inicialização do SQLite com migrações automáticas
	db, err := database.Connect(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("[LSM Server] Erro ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	router, err := api.NewRouter(cfg, db)
	if err != nil {
		log.Fatalf("[LSM Server] Erro ao inicializar roteador de produção: %v", err)
	}

	serverAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para shutdown gracioso
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[LSM Server] Servidor escutando em http://%s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[LSM Server] Falha ao iniciar servidor HTTP: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("[LSM Server] Sinal de encerramento recebido. Desligando servidor graciosamente...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[LSM Server] Encerramento forçado: %v", err)
	}

	log.Println("[LSM Server] Servidor encerrado com sucesso.")
}
