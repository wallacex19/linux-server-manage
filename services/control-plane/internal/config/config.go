package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host                string
	Port                int
	DatabasePath        string
	Environment         string
	PublicURL           string
	AllowExternalListen bool
	TrustedProxies      []string
}

func Load() *Config {
	var trustedProxies []string
	if rawProxies := getEnv("LSM_TRUSTED_PROXIES", ""); rawProxies != "" {
		for _, p := range strings.Split(rawProxies, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				trustedProxies = append(trustedProxies, trimmed)
			}
		}
	}

	cfg := &Config{
		Host:                getEnv("LSM_HOST", "127.0.0.1"),
		Port:                getEnvAsInt("LSM_PORT", 8080),
		DatabasePath:        getEnv("LSM_DB_PATH", "lsm_panel.db"),
		Environment:         getEnv("LSM_ENV", "production"),
		PublicURL:           getEnv("LSM_PUBLIC_URL", ""),
		AllowExternalListen: getEnv("LSM_ALLOW_EXTERNAL_LISTEN", "false") == "true",
		TrustedProxies:      trustedProxies,
	}
	return cfg
}

func (c *Config) Validate() error {
	env := strings.ToLower(strings.TrimSpace(c.Environment))
	if env == "" {
		env = "production"
		c.Environment = "production"
	}

	if c.Host == "" {
		c.Host = "127.0.0.1"
	}

	switch env {
	case "production", "development", "testing":
		// Permitidos
	default:
		return fmt.Errorf("ambiente LSM_ENV inválido '%s': permitido exclusivamente 'production', 'development' ou 'testing'", c.Environment)
	}

	for _, p := range c.TrustedProxies {
		if strings.Contains(p, "/") {
			if _, _, err := net.ParseCIDR(p); err != nil {
				return fmt.Errorf("LSM_TRUSTED_PROXIES contém bloco CIDR inválido '%s': %w", p, err)
			}
		} else {
			if ip := net.ParseIP(p); ip == nil {
				return fmt.Errorf("LSM_TRUSTED_PROXIES contém endereço IP inválido '%s'", p)
			}
		}
	}

	if env == "production" {
		isLoopback := c.Host == "127.0.0.1" || c.Host == "::1" || c.Host == "localhost"
		if !isLoopback && !c.AllowExternalListen {
			return errors.New("em ambiente de produção (LSM_ENV=production), LSM_HOST deve ser restrito a endereços loopback (127.0.0.1 ou ::1) para garantir o isolamento do backend através do proxy reverso Caddy; para arquiteturas de proxy distribuído, defina explicitamente LSM_ALLOW_EXTERNAL_LISTEN=true")
		}
	}

	if c.PublicURL == "" {
		if env == "production" {
			return errors.New("LSM_PUBLIC_URL é obrigatória em ambiente de produção (ex: https://painel.exemplo.com)")
		}
		c.PublicURL = fmt.Sprintf("http://localhost:%d", c.Port)
	}

	u, err := url.Parse(c.PublicURL)
	if err != nil {
		return fmt.Errorf("URL pública inválida: %w", err)
	}

	if u.User != nil {
		return errors.New("LSM_PUBLIC_URL não pode conter credenciais embutidas (user:pass)")
	}

	if env == "production" {
		if u.Scheme != "https" {
			return errors.New("em ambiente de produção (LSM_ENV=production), LSM_PUBLIC_URL deve utilizar obrigatoriamente HTTPS (ex: https://painel.exemplo.com)")
		}
	} else {
		// Em desenvolvimento ou teste, se for http://, deve ser estritamente loopback
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

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}
