package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config representa a configuração canônica única do Server Agent
type Config struct {
	Environment          string `json:"environment,omitempty"`
	PanelURL             string `json:"panel_url"`
	NodeID               string `json:"node_id"`
	AgentToken           string `json:"agent_token"`
	IdentityPath         string `json:"identity_path"`
	Hostname             string `json:"hostname"`
	PollIntervalSeconds  int    `json:"poll_interval_seconds"`
	PrivilegedHelperPath string `json:"privileged_helper_path,omitempty"`
}

// Load carrega a configuração canônica a partir de arquivo JSON e variáveis de ambiente
func Load(explicitPath string) (*Config, error) {
	cfg := &Config{
		Environment:          "production",
		IdentityPath:         "/var/lib/lsm-agent/identity.key",
		PollIntervalSeconds:  5,
		PrivilegedHelperPath: "/usr/local/libexec/lsm-privileged-helper",
	}

	if h, err := os.Hostname(); err == nil && h != "" {
		cfg.Hostname = h
	} else {
		cfg.Hostname = "linux-node"
	}

	// 1. Determina caminho do arquivo de configuração
	targetPath := explicitPath
	if targetPath == "" {
		targetPath = os.Getenv("LSM_CONFIG_PATH")
	}

	candidatePaths := []string{
		targetPath,
		"/var/lib/lsm-agent/agent_config.json",
		"./agent_config.json",
	}

	for _, p := range candidatePaths {
		if p != "" {
			// #nosec G304 -- lista controlada de caminhos canônicos de configuração
			if data, err := os.ReadFile(p); err == nil {
				if err := json.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("arquivo de configuração %s corrompido: %w", p, err)
				}
				break
			}
		}
	}

	// 2. Sobrescrita por variáveis de ambiente (sem expor segredos)
	if val := os.Getenv("LSM_ENV"); val != "" {
		cfg.Environment = val
	}
	if val := os.Getenv("LSM_PANEL_URL"); val != "" {
		cfg.PanelURL = val
	}
	if val := os.Getenv("LSM_NODE_ID"); val != "" {
		cfg.NodeID = val
	}
	if val := os.Getenv("LSM_AGENT_TOKEN"); val != "" {
		cfg.AgentToken = val
	}
	if val := os.Getenv("LSM_IDENTITY_PATH"); val != "" {
		cfg.IdentityPath = val
	}
	if val := os.Getenv("LSM_HOSTNAME"); val != "" {
		cfg.Hostname = val
	}
	if val := os.Getenv("LSM_POLL_INTERVAL"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			cfg.PollIntervalSeconds = i
		}
	}
	if val := os.Getenv("LSM_PRIVILEGED_HELPER"); val != "" {
		cfg.PrivilegedHelperPath = val
	}

	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 5
	}

	// Em desenvolvimento ou testes, define fallback de loopback se não fornecido
	if cfg.Environment != "production" && cfg.PanelURL == "" {
		cfg.PanelURL = "http://127.0.0.1:8080"
	}

	return cfg, nil
}

// Validate valida os requisitos de segurança de transporte do agente
func (c *Config) Validate() error {
	env := strings.ToLower(strings.TrimSpace(c.Environment))
	if env == "" {
		env = "production"
		c.Environment = "production"
	}

	switch env {
	case "production", "development", "testing":
		// Permitidos
	default:
		return fmt.Errorf("ambiente LSM_ENV inválido '%s': permitido exclusivamente 'production', 'development' ou 'testing'", c.Environment)
	}

	if c.PanelURL == "" {
		if env == "production" {
			return errors.New("panel_url não configurado; obrigatório informar URL HTTPS em ambiente de produção")
		}
		c.PanelURL = "http://127.0.0.1:8080"
	}

	u, err := url.Parse(c.PanelURL)
	if err != nil {
		return fmt.Errorf("URL do painel inválida: %w", err)
	}

	if u.User != nil {
		return errors.New("panel_url não pode conter credenciais embutidas (user:pass)")
	}

	if env == "production" {
		if u.Scheme != "https" {
			return errors.New("em ambiente de produção (LSM_ENV=production), panel_url deve utilizar obrigatoriamente HTTPS (ex: https://painel.exemplo.com)")
		}
	} else {
		if u.Scheme == "http" {
			host := u.Hostname()
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				return errors.New("URLs inseguras (http://) são proibidas em desenvolvimento para hosts remotos; permitido exclusivamente loopback (localhost/127.0.0.1)")
			}
		} else if u.Scheme != "https" {
			return fmt.Errorf("esquema de URL '%s' não suportado; use https:// ou http:// (loopback em dev)", u.Scheme)
		}
	}
	return nil
}

// Save grava a configuração de forma segura (permissão 0600)
func (c *Config) Save(filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, bytes, 0600)
}
