package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	VaultPath       string         `mapstructure:"vault_path" validate:"required,dir"`
	Database        DatabaseConfig `mapstructure:"database" validate:"required"`
	Sync            SyncConfig     `mapstructure:"sync"`
	IgnorePatterns  []string       `mapstructure:"ignore_patterns"`
	IncludePatterns []string       `mapstructure:"include_patterns"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	User     string `mapstructure:"user" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
	Schema   string `mapstructure:"schema"` // Optional: derived from vault name if not specified
	SSLMode  string `mapstructure:"sslmode"`
}

// SyncConfig holds sync behavior settings
type SyncConfig struct {
	DebounceMs       int `mapstructure:"debounce_ms"`
	MaxBinarySizeMB  int `mapstructure:"max_binary_size_mb"`
	BatchSize        int `mapstructure:"batch_size"`
	RetryAttempts    int `mapstructure:"retry_attempts"`
	RetryDelayMs     int `mapstructure:"retry_delay_ms"`
}

// ConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) ConnectionString() string {
	sslMode := d.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Database, sslMode,
	)
	// Set search_path to use the vault's schema
	if d.Schema != "" {
		connStr += "&search_path=" + d.Schema + ",public"
	}
	return connStr
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Port:    5432,
			SSLMode: "require",
		},
		Sync: SyncConfig{
			DebounceMs:      2000,
			MaxBinarySizeMB: 50,
			BatchSize:       100,
			RetryAttempts:   3,
			RetryDelayMs:    1000,
		},
		IgnorePatterns: []string{
			".obsidian/**",
			".trash/**",
			".git/**",
			"**/.DS_Store",
			"**/node_modules/**",
		},
	}
}

// Load reads configuration from file and environment
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("database.port", defaults.Database.Port)
	v.SetDefault("database.sslmode", defaults.Database.SSLMode)
	v.SetDefault("sync.debounce_ms", defaults.Sync.DebounceMs)
	v.SetDefault("sync.max_binary_size_mb", defaults.Sync.MaxBinarySizeMB)
	v.SetDefault("sync.batch_size", defaults.Sync.BatchSize)
	v.SetDefault("sync.retry_attempts", defaults.Sync.RetryAttempts)
	v.SetDefault("sync.retry_delay_ms", defaults.Sync.RetryDelayMs)
	v.SetDefault("ignore_patterns", defaults.IgnorePatterns)

	// Configure config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Search for config in standard locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath(getConfigDir())
	}

	// Enable environment variable substitution
	v.AutomaticEnv()
	v.SetEnvPrefix("OBSYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is okay if we have environment variables
	}

	// Unmarshal into struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Expand environment variables in password
	cfg.Database.Password = os.ExpandEnv(cfg.Database.Password)

	// Expand vault path
	cfg.VaultPath = expandPath(cfg.VaultPath)

	// Derive schema name from vault folder if not specified
	if cfg.Database.Schema == "" {
		cfg.Database.Schema = SanitizeIdentifier(filepath.Base(cfg.VaultPath))
	}

	// Validate
	validate := validator.New()

	// Register custom validation for directory existence
	validate.RegisterValidation("dir", func(fl validator.FieldLevel) bool {
		path := fl.Field().String()
		if path == "" {
			return false
		}
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		return info.IsDir()
	})

	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// getConfigDir returns the appropriate config directory for the OS
func getConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "obsync-pg")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".config", "obsync-pg")
	default:
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "obsync-pg")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "obsync-pg")
	}
}

// GetStateDir returns the directory for storing state files
func GetStateDir() (string, error) {
	dir := getConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}
	return dir, nil
}

// expandPath expands ~ and environment variables in a path
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}

// SanitizeIdentifier converts a vault name into a valid PostgreSQL identifier (schema/database name)
// Rules:
// - Lowercase only
// - Starts with letter or underscore
// - Contains only letters, digits, underscores
// - Spaces and hyphens become underscores
// - Max 63 characters (PostgreSQL limit)
func SanitizeIdentifier(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and hyphens with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any character that isn't alphanumeric or underscore
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	name = reg.ReplaceAllString(name, "")

	// Collapse multiple underscores
	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	// Trim leading/trailing underscores
	name = strings.Trim(name, "_")

	// Ensure it starts with a letter (prepend 'vault_' if it starts with digit or is empty)
	if len(name) == 0 {
		name = "vault"
	} else if unicode.IsDigit(rune(name[0])) {
		name = "vault_" + name
	}

	// PostgreSQL max identifier length is 63 characters
	if len(name) > 63 {
		name = name[:63]
		// Make sure we don't end with underscore after truncation
		name = strings.TrimRight(name, "_")
	}

	return name
}
