package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/linux-server-manager/agent/internal/config"
	"github.com/linux-server-manager/agent/internal/identity"
	"github.com/linux-server-manager/agent/internal/transport"
)

func main() {
	configPathFlag := flag.String("config", "", "Caminho do arquivo de configuração JSON do LSM Agent")
	flag.StringVar(configPathFlag, "c", "", "Caminho do arquivo de configuração (atalho)")

	identityPathFlag := flag.String("identity-path", "", "Caminho do arquivo de chave Ed25519")
	nodeIDFlag := flag.String("node-id", "", "ID do nó para vinculação da mensagem canônica de pareamento")
	exportPubFlag := flag.Bool("export-pubkey", false, "Exibe a chave pública Ed25519 em hexadecimal e sai")
	signTokenFlag := flag.String("sign-token", "", "Assina o token de pareamento com a chave privada Ed25519 e sai")

	flag.Parse()

	// Operações de auxílio criptográfico para o instalador/provisionamento
	if *exportPubFlag || *signTokenFlag != "" {
		idPath := *identityPathFlag
		if idPath == "" {
			idPath = "/var/lib/lsm-agent/identity.key"
		}
		ident, err := identity.LoadOrCreateIdentity(idPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao carregar/gerar identidade: %v\n", err)
			os.Exit(1)
		}
		if *exportPubFlag {
			fmt.Println(ident.PublicKeyHex())
			return
		}
		if *signTokenFlag != "" {
			var msgToSign []byte
			if *nodeIDFlag != "" {
				msgToSign = []byte(fmt.Sprintf("LSM-PAIR-v1:%s:%s", *nodeIDFlag, *signTokenFlag))
			} else {
				msgToSign = []byte(fmt.Sprintf("LSM-PAIR-v1:%s", *signTokenFlag))
			}
			sig := ident.SignMessage(msgToSign)
			fmt.Println(hex.EncodeToString(sig))
			return
		}
	}

	cfg, err := config.Load(*configPathFlag)
	if err != nil {
		log.Fatalf("[LSM Agent] Falha ao carregar configuração: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("[LSM Agent] Erro de validação de configuração: %v", err)
	}

	log.Printf("[LSM Agent] Iniciando agente Linux Server Manager (Hostname: %s, Painel: %s)...", cfg.Hostname, cfg.PanelURL)

	// Carrega ou gera identidade criptográfica do nó
	ident, err := identity.LoadOrCreateIdentity(cfg.IdentityPath)
	if err != nil {
		log.Fatalf("[LSM Agent] Falha crítica ao inicializar identidade criptográfica: %v", err)
	}

	log.Printf("[LSM Agent] Identidade carregada. NodeID: %s | Chave Pública: %s", ident.NodeID, ident.PublicKeyHex())

	if cfg.NodeID == "" || cfg.AgentToken == "" {
		log.Printf("[LSM Agent] AVISO: Nó ainda não pareado (NodeID ou AgentToken ausentes).")
		log.Printf("[LSM Agent] Para parear, utilize o instalador com o token de pareamento gerado no painel.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := transport.NewClient(*cfg, ident)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Inicia o cliente de transporte WSS com reconexão automática e backoff
	go func() {
		client.Start(ctx)
	}()

	log.Println("[LSM Agent] Agente em execução. Pressione Ctrl+C para encerrar.")

	<-shutdownChan
	log.Println("[LSM Agent] Sinal de término recebido. Encerrando agente graciosamente...")
	cancel()
	client.Stop()
	log.Println("[LSM Agent] Agente finalizado com sucesso.")
}
