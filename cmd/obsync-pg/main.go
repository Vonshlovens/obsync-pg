package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/deveric/obsync-pg/internal/config"
	"github.com/deveric/obsync-pg/internal/db"
	"github.com/deveric/obsync-pg/internal/sync"
	"github.com/deveric/obsync-pg/internal/watcher"
)

var (
	cfgFile string
	verbose bool
	version = "dev"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "obsync-pg",
		Short:   "Obsidian vault sync daemon for Postgres",
		Long:    `A cross-platform daemon that monitors an Obsidian vault and syncs changes to a remote PostgreSQL database.`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Setup logging
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")

	rootCmd.AddCommand(
		daemonCmd(),
		syncCmd(),
		statusCmd(),
		migrateCmd(),
		initCmd(),
		pullCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Start the background watcher/sync process",
		Long:  `Starts a daemon that watches the Obsidian vault for changes and syncs them to the database in real-time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			database, err := db.New(ctx, &cfg.Database)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			engine, err := sync.NewEngine(database, cfg)
			if err != nil {
				return fmt.Errorf("failed to create sync engine: %w", err)
			}

			// Perform initial full sync
			slog.Info("performing initial sync")
			if err := engine.FullReconcile(ctx); err != nil {
				slog.Error("initial sync failed", "error", err)
			}

			// Start file watcher
			w, err := watcher.NewWatcher(cfg.VaultPath, cfg.Sync.DebounceMs, cfg.IgnorePatterns, cfg.IncludePatterns)
			if err != nil {
				return fmt.Errorf("failed to create watcher: %w", err)
			}

			if err := w.Start(ctx); err != nil {
				return fmt.Errorf("failed to start watcher: %w", err)
			}

			// Handle graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			slog.Info("daemon started", "vault", cfg.VaultPath)
			fmt.Println("Watching vault for changes. Press Ctrl+C to stop.")

			// Periodic state save and retry ticker
			saveTicker := time.NewTicker(30 * time.Second)
			defer saveTicker.Stop()

			for {
				select {
				case <-sigCh:
					slog.Info("shutting down...")
					w.Stop()
					w.Flush()
					engine.SaveState()
					return nil

				case event := <-w.Events():
					slog.Debug("file event", "path", event.Path, "type", event.EventType)
					if err := engine.SyncFile(ctx, event.Path, event.EventType); err != nil {
						slog.Error("sync failed", "path", event.Path, "error", err)
					}

				case <-saveTicker.C:
					engine.SaveState()
					engine.RetryFailed(ctx)
				}
			}
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "One-time full sync, then exit",
		Long:  `Performs a full synchronization of the vault to the database and exits.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			database, err := db.New(ctx, &cfg.Database)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			engine, err := sync.NewEngine(database, cfg)
			if err != nil {
				return fmt.Errorf("failed to create sync engine: %w", err)
			}

			if err := engine.FullReconcile(ctx); err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			if err := engine.SaveState(); err != nil {
				slog.Warn("failed to save state", "error", err)
			}

			fmt.Println("Sync completed successfully.")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show connection status and sync info",
		Long:  `Shows the current database connection status, last sync time, and file counts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			database, err := db.New(ctx, &cfg.Database)
			if err != nil {
				fmt.Printf("Database Status: Disconnected\n")
				fmt.Printf("Error: %v\n", err)
				return nil
			}
			defer database.Close()

			status, err := database.GetStatus(ctx)
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			fmt.Println("=== Obsync-PG Status ===")
			fmt.Printf("Database Status: Connected\n")
			fmt.Printf("  Host: %s\n", cfg.Database.Host)
			fmt.Printf("  Database: %s\n", cfg.Database.Database)
			fmt.Printf("  Schema: %s\n", cfg.Database.Schema)
			fmt.Println()
			fmt.Printf("Vault Path: %s\n", cfg.VaultPath)
			fmt.Println()
			fmt.Printf("Synced Files:\n")
			fmt.Printf("  Notes: %d\n", status.TotalNotes)
			fmt.Printf("  Attachments: %d\n", status.TotalAttach)
			if status.LastSyncTime != nil {
				fmt.Printf("  Last Sync: %s\n", status.LastSyncTime.Format(time.RFC3339))
			}

			return nil
		},
	}
}

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long:  `Runs all pending database migrations.`,
	}

	migrationsDir := ""
	cmd.Flags().StringVar(&migrationsDir, "dir", "migrations", "migrations directory")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		database, err := db.New(ctx, &cfg.Database)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer database.Close()

		// Resolve migrations directory
		if !filepath.IsAbs(migrationsDir) {
			// Try relative to executable first
			exe, _ := os.Executable()
			exeDir := filepath.Dir(exe)
			if _, err := os.Stat(filepath.Join(exeDir, migrationsDir)); err == nil {
				migrationsDir = filepath.Join(exeDir, migrationsDir)
			} else {
				// Try relative to current directory
				cwd, _ := os.Getwd()
				migrationsDir = filepath.Join(cwd, migrationsDir)
			}
		}

		if err := database.RunMigrations(ctx, migrationsDir); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		fmt.Println("Migrations completed successfully.")
		return nil
	}

	return cmd
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup to create config file",
		Long:  `Interactively creates a configuration file and tests the database connection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			fmt.Println("=== Obsync-PG Setup ===")
			fmt.Println()

			// Get vault path
			fmt.Print("Obsidian vault path: ")
			vaultPath, _ := reader.ReadString('\n')
			vaultPath = strings.TrimSpace(vaultPath)

			// Validate vault path
			if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
				return fmt.Errorf("vault path does not exist: %s", vaultPath)
			}

			// Get database settings
			fmt.Println("\nDatabase Configuration:")
			fmt.Print("  Host: ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)

			fmt.Print("  Port [5432]: ")
			portStr, _ := reader.ReadString('\n')
			portStr = strings.TrimSpace(portStr)
			if portStr == "" {
				portStr = "5432"
			}

			fmt.Print("  User: ")
			user, _ := reader.ReadString('\n')
			user = strings.TrimSpace(user)

			fmt.Print("  Password: ")
			password, _ := reader.ReadString('\n')
			password = strings.TrimSpace(password)

			fmt.Print("  Database name: ")
			dbName, _ := reader.ReadString('\n')
			dbName = strings.TrimSpace(dbName)
			if dbName == "" {
				return fmt.Errorf("database name is required")
			}

			// Derive default schema name from vault folder
			defaultSchema := config.SanitizeIdentifier(filepath.Base(vaultPath))
			fmt.Printf("  Schema name [%s]: ", defaultSchema)
			schemaName, _ := reader.ReadString('\n')
			schemaName = strings.TrimSpace(schemaName)
			if schemaName == "" {
				schemaName = defaultSchema
			}

			fmt.Print("  SSL mode [require]: ")
			sslMode, _ := reader.ReadString('\n')
			sslMode = strings.TrimSpace(sslMode)
			if sslMode == "" {
				sslMode = "require"
			}

			// Generate config content
			configContent := fmt.Sprintf(`vault_path: "%s"

database:
  host: "%s"
  port: %s
  user: "%s"
  password: "${DB_PASSWORD}"  # Set DB_PASSWORD environment variable
  database: "%s"
  schema: "%s"  # Each vault gets its own schema
  sslmode: "%s"

sync:
  debounce_ms: 2000
  max_binary_size_mb: 50
  batch_size: 100

ignore_patterns:
  - ".obsidian/**"
  - ".trash/**"
  - ".git/**"
  - "**/.DS_Store"
  - "**/node_modules/**"
`, vaultPath, host, portStr, user, dbName, schemaName, sslMode)

			// Determine config path
			configDir, err := config.GetStateDir()
			if err != nil {
				return err
			}
			configPath := filepath.Join(configDir, "config.yaml")

			// Write config file
			if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("\nConfig file written to: %s\n", configPath)
			fmt.Printf("\nIMPORTANT: Set the DB_PASSWORD environment variable:\n")
			fmt.Printf("  export DB_PASSWORD='%s'\n", password)
			fmt.Println("\nTo test the connection, run: obsync-pg status")
			fmt.Println("To run migrations, run: obsync-pg migrate")
			fmt.Println("To start syncing, run: obsync-pg daemon")

			return nil
		},
	}
}

func pullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Download files from database to local vault",
		Long:  `Downloads all files from the database to the local vault. Use this to set up a new device with existing vault data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check if vault directory exists, create if not
			if _, err := os.Stat(cfg.VaultPath); os.IsNotExist(err) {
				fmt.Printf("Creating vault directory: %s\n", cfg.VaultPath)
				if err := os.MkdirAll(cfg.VaultPath, 0755); err != nil {
					return fmt.Errorf("failed to create vault directory: %w", err)
				}
			}

			database, err := db.New(ctx, &cfg.Database)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			engine, err := sync.NewEngine(database, cfg)
			if err != nil {
				return fmt.Errorf("failed to create sync engine: %w", err)
			}

			if err := engine.PullFromDB(ctx); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}

			fmt.Println("Pull completed successfully.")
			return nil
		},
	}
}
