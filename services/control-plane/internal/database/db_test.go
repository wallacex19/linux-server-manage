package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lsm_test_*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar e migrar banco: %v", err)
	}
	defer db.Close()

	// Verifica se a tabela nodes existe
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='nodes'").Scan(&tableName)
	if err != nil {
		t.Fatalf("tabela 'nodes' não encontrada após migração: %v", err)
	}

	if tableName != "nodes" {
		t.Errorf("esperado nome de tabela 'nodes', obtido '%s'", tableName)
	}

	// Verifica registro da migração
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations WHERE version=1").Scan(&version)
	if err != nil {
		t.Fatalf("registro da migração 1 não encontrado: %v", err)
	}

	if version != 1 {
		t.Errorf("esperada versão 1, obtida %d", version)
	}
}
