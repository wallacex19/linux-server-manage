package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/000001_init_schema.sql
var initialMigration string

//go:embed migrations/000002_game_servers.sql
var gameServersMigration string

//go:embed migrations/000003_multi_server.sql
var multiServerMigration string

type DB struct {
	*sql.DB
}

func Connect(dbPath string) (*DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite gerencia concorrência com WAL de forma confiável com 1 writer ou conexões limitadas
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("falha ao conectar ao sqlite: %w", err)
	}

	appDB := &DB{db}
	if err := appDB.runMigrations(); err != nil {
		return nil, fmt.Errorf("falha ao executar migrações: %w", err)
	}

	return appDB, nil
}

func (d *DB) runMigrations() error {
	// Cria tabela de controle de migração se não existir
	_, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("falha ao criar schema_migrations: %w", err)
	}

	var count int
	err = d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		return fmt.Errorf("falha ao checar versao de migracao: %w", err)
	}

	if count == 0 {
		log.Println("[Database] Executando migração 000001_init_schema.sql...")
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(initialMigration); err != nil {
			return fmt.Errorf("falha ao aplicar migração 1: %w", err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (1)"); err != nil {
			return fmt.Errorf("falha ao registrar versao de migracao: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		log.Println("[Database] Migração 000001_init_schema.sql aplicada com sucesso.")
	}

	var count2 int
	err = d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 2").Scan(&count2)
	if err != nil {
		return fmt.Errorf("falha ao checar versao de migracao 2: %w", err)
	}

	if count2 == 0 {
		log.Println("[Database] Executando migração 000002_game_servers.sql...")
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(gameServersMigration); err != nil {
			return fmt.Errorf("falha ao aplicar migração 2: %w", err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (2)"); err != nil {
			return fmt.Errorf("falha ao registrar versao de migracao 2: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		log.Println("[Database] Migração 000002_game_servers.sql aplicada com sucesso.")
	}

	var count3 int
	err = d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 3").Scan(&count3)
	if err != nil {
		return fmt.Errorf("falha ao checar versao de migracao 3: %w", err)
	}

	if count3 == 0 {
		log.Println("[Database] Executando migração 000003_multi_server.sql...")
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(multiServerMigration); err != nil {
			return fmt.Errorf("falha ao aplicar migração 3: %w", err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (3)"); err != nil {
			return fmt.Errorf("falha ao registrar versao de migracao 3: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		log.Println("[Database] Migração 000003_multi_server.sql aplicada com sucesso.")
	}

	return nil
}
