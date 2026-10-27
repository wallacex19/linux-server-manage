package transport_test

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestTLS_RejectInvalidOrUntrustedCertificate(t *testing.T) {
	// Servidor HTTPS/WSS de teste com certificado autoassinado (não assinado por CA confiável do SO)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err == nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("ok"))
				_ = conn.Close()
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	serverAddr := ts.Listener.Addr().String()
	wsURL := "wss://" + serverAddr

	// =========================================================================
	// Etapa 1: Validação do Handshake TLS Direto
	// Prova especificamente que o erro decorre de validação de certificado (x509),
	// e NÃO de encerramento prematuro de conexão ou erro genérico de socket.
	// =========================================================================
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConn, tlsErr := tls.DialWithDialer(dialer, "tcp", serverAddr, &tls.Config{
		InsecureSkipVerify: false,
	})
	if tlsConn != nil {
		_ = tlsConn.Close()
	}
	if tlsErr == nil {
		t.Fatal("FALHA DE SEGURANÇA: Handshake TLS direto teve sucesso com certificado autoassinado e InsecureSkipVerify=false!")
	}

	// Verifica categoricamente que tlsErr é um erro de validação de autoridade certificadora x509
	var unknownAuthErr x509.UnknownAuthorityError
	var certInvalidErr x509.CertificateInvalidError
	var hostnameErr x509.HostnameError

	isCertErr := errors.As(tlsErr, &unknownAuthErr) ||
		errors.As(tlsErr, &certInvalidErr) ||
		errors.As(tlsErr, &hostnameErr) ||
		strings.Contains(strings.ToLower(tlsErr.Error()), "unknown authority") ||
		strings.Contains(strings.ToLower(tlsErr.Error()), "certificate") ||
		strings.Contains(strings.ToLower(tlsErr.Error()), "x509")

	if !isCertErr {
		t.Fatalf("esperado erro explícito de validação de certificado x509, obtido erro genérico: %v", tlsErr)
	}

	// =========================================================================
	// Etapa 2: Contra-prova Positiva com CA Confiável
	// Comprova que o socket, porta e servidor estão plenamente funcionais:
	// quando o certificado é adicionado a uma pool confiável, o handshake tem sucesso.
	// =========================================================================
	trustedPool := x509.NewCertPool()
	trustedPool.AddCert(ts.Certificate())

	validTLSConn, validTLSErr := tls.DialWithDialer(dialer, "tcp", serverAddr, &tls.Config{
		RootCAs:            trustedPool,
		InsecureSkipVerify: false,
	})
	if validTLSErr != nil {
		t.Fatalf("contra-prova falhou: handshake TLS deveria ter sucesso com certificado confiável: %v", validTLSErr)
	}
	_ = validTLSConn.Close()

	// =========================================================================
	// Etapa 3: Validação do Dialer WebSocket de Produção
	// Prova que o cliente WebSocket oficial rejeita o certificado não-confiável.
	// =========================================================================
	prodDialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}

	wsConn, _, wsErr := prodDialer.Dial(wsURL, nil)
	if wsConn != nil {
		_ = wsConn.Close()
	}
	if wsErr == nil {
		t.Fatal("FALHA DE SEGURANÇA: Dialer WebSocket de produção conectou com sucesso a servidor com certificado autoassinado!")
	}

	// Prova que quando o certificado confiável é injetado, o mesmo Dialer WebSocket conecta perfeitamente
	trustedDialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs:            trustedPool,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}
	trustedWSConn, _, trustedWSErr := trustedDialer.Dial(wsURL, nil)
	if trustedWSErr != nil {
		t.Fatalf("contra-prova falhou: Dialer WebSocket deveria conectar com sucesso quando CA é confiável: %v", trustedWSErr)
	}
	_ = trustedWSConn.Close()
}
