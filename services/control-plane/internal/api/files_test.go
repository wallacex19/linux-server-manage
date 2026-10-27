package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/linux-server-manager/backend/internal/files"
)

func TestFilesEndpoints(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "lsm_api_files_*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempHome)

	router := setupTestRouter()

	// 1. Login com admin
	loginPayload := LoginRequest{
		Username: "admin",
		Password: "adminpassword",
	}
	body, _ := json.Marshal(loginPayload)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "lsm_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("cookie de sessão não retornado")
	}

	// 2. Cria arquivo de teste
	createPayload := PathRequest{
		Path: filepath.Join(tempHome, "server_config.yaml"),
	}
	createBody, _ := json.Marshal(createPayload)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/files/create", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(sessionCookie)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /files/create, obtido %d: %s", createW.Code, createW.Body.String())
	}

	// 3. Salva conteúdo
	savePayload := SaveFileRequest{
		Path:    filepath.Join(tempHome, "server_config.yaml"),
		Content: "port: 27015\nmaxplayers: 32\n",
	}
	saveBody, _ := json.Marshal(savePayload)
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/files/save", bytes.NewReader(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.AddCookie(sessionCookie)
	saveW := httptest.NewRecorder()
	router.ServeHTTP(saveW, saveReq)

	if saveW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /files/save, obtido %d", saveW.Code)
	}

	// 4. Lê arquivo
	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/read?path="+filepath.Join(tempHome, "server_config.yaml"), nil)
	readReq.AddCookie(sessionCookie)
	readW := httptest.NewRecorder()
	router.ServeHTTP(readW, readReq)

	if readW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /files/read, obtido %d", readW.Code)
	}

	var fileContent files.FileContent
	if err := json.Unmarshal(readW.Body.Bytes(), &fileContent); err != nil {
		t.Fatalf("erro ao decodificar FileContent: %v", err)
	}

	if fileContent.Content != "port: 27015\nmaxplayers: 32\n" {
		t.Errorf("conteúdo lido diverge do salvo: %s", fileContent.Content)
	}

	// 5. Lista pasta
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/list?path="+tempHome, nil)
	listReq.AddCookie(sessionCookie)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /files/list, obtido %d", listW.Code)
	}

	var listing files.DirectoryListing
	if err := json.Unmarshal(listW.Body.Bytes(), &listing); err != nil {
		t.Fatalf("erro ao decodificar DirectoryListing: %v", err)
	}

	if len(listing.Entries) != 1 {
		t.Errorf("esperado 1 arquivo listado, obtido %d", len(listing.Entries))
	}
}
