package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/vonshlovens/obsync-pg/internal/config"
)

// DB wraps the database connection pool
type DB struct {
	Pool   *pgxpool.Pool
	config *config.DatabaseConfig
	Schema string
}

// New creates a new database connection pool
func New(ctx context.Context, cfg *config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("connected to database",
		"host", cfg.Host,
		"database", cfg.Database,
		"schema", cfg.Schema)

	return &DB{
		Pool:   pool,
		config: cfg,
		Schema: cfg.Schema,
	}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		slog.Info("database connection closed")
	}
}

// Ping checks if the database is reachable
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// EnsureSchema creates the schema if it doesn't exist
func (db *DB) EnsureSchema(ctx context.Context) error {
	if db.Schema == "" {
		return nil
	}

	// Create schema if it doesn't exist
	_, err := db.Pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", db.Schema))
	if err != nil {
		return fmt.Errorf("failed to create schema %s: %w", db.Schema, err)
	}

	slog.Info("schema ready", "schema", db.Schema)
	return nil
}

// RunMigrations executes all pending database migrations
func (db *DB) RunMigrations(ctx context.Context, migrationsDir string) error {
	// Ensure schema exists first
	if err := db.EnsureSchema(ctx); err != nil {
		return err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	stdDB, err := sql.Open("pgx", db.config.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to open stdlib connection: %w", err)
	}
	defer stdDB.Close()

	// Set goose table name to be schema-specific to avoid conflicts
	if db.Schema != "" {
		goose.SetTableName(db.Schema + ".goose_db_version")
	}

	if err := goose.Up(stdDB, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("migrations completed successfully", "schema", db.Schema)
	return nil
}

// MigrationStatus returns the current migration status
func (db *DB) MigrationStatus(migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	stdDB, err := sql.Open("pgx", db.config.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to open stdlib connection: %w", err)
	}
	defer stdDB.Close()

	// Set goose table name to be schema-specific
	if db.Schema != "" {
		goose.SetTableName(db.Schema + ".goose_db_version")
	}

	return goose.Status(stdDB, migrationsDir)
}

// GetStatus returns the current sync status
func (db *DB) GetStatus(ctx context.Context) (*SyncStatus, error) {
	status := &SyncStatus{
		Connected: true,
	}

	// Count notes
	var noteCount int
	err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM vault_notes").Scan(&noteCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count notes: %w", err)
	}
	status.TotalNotes = noteCount

	// Count attachments
	var attachCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM vault_attachments").Scan(&attachCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count attachments: %w", err)
	}
	status.TotalAttach = attachCount

	// Get last sync time
	var lastSync *time.Time
	err = db.Pool.QueryRow(ctx, `
		SELECT MAX(synced_at) FROM (
			SELECT synced_at FROM vault_notes
			UNION ALL
			SELECT synced_at FROM vault_attachments
		) t
	`).Scan(&lastSync)
	if err != nil {
		slog.Warn("failed to get last sync time", "error", err)
	}
	status.LastSyncTime = lastSync

	return status, nil
}
