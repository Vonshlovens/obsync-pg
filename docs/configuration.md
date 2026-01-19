# Configuration Reference

This document describes all configuration options for Obsync-PG.

## Config File Location

The config file is automatically loaded from:

| OS | Path |
|----|------|
| Linux | `~/.config/obsync-pg/config.yaml` |
| macOS | `~/.config/obsync-pg/config.yaml` |
| Windows | `%APPDATA%\obsync-pg\config.yaml` |

You can also specify a custom path with the `-c` flag:

```bash
obsync-pg -c /path/to/config.yaml daemon
```

## Full Configuration Example

```yaml
# Path to your Obsidian vault (required)
vault_path: "/Users/you/Documents/ObsidianVault"

# Database connection settings
database:
  host: "db.xxx.supabase.co"      # Required
  port: 5432                       # Required (default: 5432)
  user: "postgres"                 # Required
  password: "${DB_PASSWORD}"       # Required (supports env var expansion)
  database: "postgres"             # Required
  schema: "my_vault"               # Optional (derived from vault name if omitted)
  sslmode: "require"               # Optional (default: require)

# Sync behavior settings
sync:
  debounce_ms: 2000                # Wait time after last change (default: 2000)
  max_binary_size_mb: 50           # Skip attachments larger than this (default: 50)
  batch_size: 100                  # Files per transaction (default: 100)
  retry_attempts: 3                # Retries for failed operations (default: 3)
  retry_delay_ms: 1000             # Delay between retries (default: 1000)

# Files/folders to ignore (glob patterns)
ignore_patterns:
  - ".obsidian/**"                 # Obsidian config
  - ".trash/**"                    # Obsidian trash
  - ".git/**"                      # Git repository
  - "**/.DS_Store"                 # macOS files
  - "**/node_modules/**"           # Node.js

# Optional: Only sync specific folders
# include_patterns:
#   - "notes/**"
#   - "projects/**"
```

## Configuration Options

### vault_path (required)

The absolute path to your Obsidian vault folder.

```yaml
vault_path: "/Users/you/Documents/MyVault"
```

Supports:
- Environment variable expansion: `${HOME}/Documents/MyVault`
- Home directory shortcut: `~/Documents/MyVault`

### database (required)

PostgreSQL connection settings.

#### database.host (required)

The PostgreSQL server hostname.

```yaml
database:
  host: "db.xxx.supabase.co"    # Supabase
  host: "localhost"              # Local PostgreSQL
  host: "192.168.1.100"         # IP address
```

#### database.port (required)

PostgreSQL port number. Default: `5432`

```yaml
database:
  port: 5432     # Standard PostgreSQL / Supabase direct
  port: 6543     # Supabase pooler (not recommended for daemon)
```

#### database.user (required)

Database username.

```yaml
database:
  user: "postgres"
```

#### database.password (required)

Database password. **Always use environment variable expansion for security.**

```yaml
database:
  password: "${DB_PASSWORD}"           # Recommended: env var
  password: "${SUPABASE_DB_PASSWORD}"  # Custom env var name
```

Then set the environment variable:
```bash
export DB_PASSWORD='your-actual-password'
```

#### database.database (required)

The database name to connect to.

```yaml
database:
  database: "postgres"         # Supabase default
  database: "obsidian_sync"    # Custom database
```

#### database.schema (optional)

The schema name for this vault's tables. If omitted, derived from the vault folder name.

```yaml
database:
  schema: "my_vault"           # Explicit schema
  # schema omitted             # Auto-derived from vault_path
```

**Auto-derivation rules:**
| Vault Folder | Schema Name |
|--------------|-------------|
| `My Obsidian Vault` | `my_obsidian_vault` |
| `Work Notes` | `work_notes` |
| `2024 Journal` | `vault_2024_journal` |

#### database.sslmode (optional)

SSL connection mode. Default: `require`

```yaml
database:
  sslmode: "require"       # Require SSL (recommended, Supabase default)
  sslmode: "disable"       # No SSL (local development only)
  sslmode: "verify-ca"     # Verify server certificate
  sslmode: "verify-full"   # Verify server certificate and hostname
```

### sync (optional)

Sync behavior settings.

#### sync.debounce_ms

Milliseconds to wait after the last file change before syncing. Prevents excessive syncing during rapid edits.

```yaml
sync:
  debounce_ms: 2000    # Default: 2 seconds
  debounce_ms: 5000    # 5 seconds for slower connections
  debounce_ms: 500     # Faster sync (more DB operations)
```

#### sync.max_binary_size_mb

Maximum size in MB for binary attachments. Files larger than this are skipped.

```yaml
sync:
  max_binary_size_mb: 50     # Default: 50 MB
  max_binary_size_mb: 100    # Allow larger files
  max_binary_size_mb: 10     # Strict limit for small databases
```

#### sync.batch_size

Number of files to process per database transaction during bulk operations.

```yaml
sync:
  batch_size: 100    # Default
  batch_size: 50     # Smaller batches for reliability
  batch_size: 200    # Larger batches for speed
```

#### sync.retry_attempts

Number of times to retry failed sync operations.

```yaml
sync:
  retry_attempts: 3    # Default
  retry_attempts: 5    # More retries for unreliable connections
```

#### sync.retry_delay_ms

Milliseconds to wait between retry attempts.

```yaml
sync:
  retry_delay_ms: 1000    # Default: 1 second
  retry_delay_ms: 5000    # Longer delay for rate-limited servers
```

### ignore_patterns (optional)

Glob patterns for files and folders to exclude from syncing.

```yaml
ignore_patterns:
  - ".obsidian/**"         # Obsidian config folder
  - ".trash/**"            # Obsidian trash
  - ".git/**"              # Git repository
  - "**/.DS_Store"         # macOS system files
  - "**/node_modules/**"   # Node.js dependencies
  - "Archive/**"           # Custom folder to ignore
  - "**/*.tmp"             # All .tmp files
  - "Private/**"           # Private folder
```

**Pattern syntax:**
- `*` matches any sequence of characters (not including `/`)
- `**` matches any sequence of characters (including `/`)
- `?` matches any single character
- `[abc]` matches any character in the set

### include_patterns (optional)

If specified, **only** files matching these patterns are synced. Useful for syncing specific folders only.

```yaml
include_patterns:
  - "notes/**"
  - "projects/**"
  - "journal/**"
```

**Note:** When `include_patterns` is set, files not matching any pattern are ignored, even if they don't match `ignore_patterns`.

## Environment Variables

All config values can also be set via environment variables with the `OBSYNC_` prefix:

```bash
export OBSYNC_VAULT_PATH="/path/to/vault"
export OBSYNC_DATABASE_HOST="localhost"
export OBSYNC_DATABASE_PORT="5432"
export OBSYNC_SYNC_DEBOUNCE_MS="3000"
```

Environment variables override config file values.

## State File

Obsync-PG maintains a local state file to track synced files:

| OS | Path |
|----|------|
| Linux/macOS | `~/.config/obsync-pg/state-<vault-hash>.json` |
| Windows | `%APPDATA%\obsync-pg\state-<vault-hash>.json` |

This file is automatically managed and shouldn't be edited manually. Delete it to force a full re-sync.
