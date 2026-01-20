# Obsync-PG

A cross-platform Go application that monitors an Obsidian vault folder and syncs changes to a remote PostgreSQL database (compatible with Supabase, Hostinger VPS, or any Postgres instance).

## Features

- Real-time file watching with intelligent debouncing
- Full YAML frontmatter parsing with support for custom fields
- Automatic extraction of wikilinks (`[[Page Name]]`) and inline tags (`#tag`)
- Binary attachment storage (images, PDFs, etc.)
- Multi-device support with pull command for new device setup
- Incremental sync with SHA256 hash-based change detection
- Cross-platform builds (macOS, Linux, Windows)

## Quick Start

### 1. Install

```bash
# From source
go install github.com/vonshlovens/obsync-pg/cmd/obsync-pg@latest

# Or build from source
git clone https://github.com/vonshlovens/obsync-pg.git
cd obsync-pg
make build
```

### 2. Setup Database

Create a PostgreSQL database:

```sql
CREATE DATABASE obsidian_sync;
```

### 3. Configure

Run the interactive setup:

```bash
obsync-pg init
```

Or create a config file manually at:
- Linux/macOS: `~/.config/obsync-pg/config.yaml`
- Windows: `%APPDATA%\obsync-pg\config.yaml`

See `config.example.yaml` for all options.

### 4. Run Migrations

```bash
export DB_PASSWORD='your-password'
obsync-pg migrate
```

### 5. Start Syncing

```bash
# One-time sync
obsync-pg sync

# Or run as daemon
obsync-pg daemon
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `obsync-pg daemon` | Start the background watcher/sync process |
| `obsync-pg sync` | One-time full sync, then exit |
| `obsync-pg status` | Show connection status and sync info |
| `obsync-pg migrate` | Run database migrations |
| `obsync-pg init` | Interactive setup wizard |
| `obsync-pg pull` | Download files from database to local vault (for new devices) |

### Flags

- `-c, --config <path>` - Path to config file
- `-v, --verbose` - Enable debug logging

## Configuration

```yaml
vault_path: "/path/to/your/vault"

database:
  host: "your-host"
  port: 5432
  user: "postgres"
  password: "${DB_PASSWORD}"  # Uses environment variable
  database: "obsidian_sync"   # Your database name
  # schema: "my_vault"        # Optional: auto-derived from vault folder name
  sslmode: "require"

sync:
  debounce_ms: 2000         # Wait after last change before syncing
  max_binary_size_mb: 50    # Skip large binary files
  batch_size: 100           # Files per transaction

ignore_patterns:
  - ".obsidian/**"
  - ".trash/**"
  - ".git/**"
```

### Schema-based Vault Isolation

Each vault gets its own PostgreSQL schema within the same database. If `schema` is not specified, it's automatically derived from the vault folder name:

| Vault Folder | Schema Name |
|--------------|-------------|
| `My Obsidian Vault` | `my_obsidian_vault` |
| `Work Notes` | `work_notes` |
| `2024 Journal` | `vault_2024_journal` |
| `Notes & Ideas` | `notes_ideas` |

This allows running multiple vaults on the same database (ideal for Supabase which uses a single database). Each vault's tables are isolated in their own schema:
- `my_obsidian_vault.vault_notes`
- `my_obsidian_vault.vault_attachments`
- `work_notes.vault_notes`
- etc.

## Database Schema

### vault_notes

Stores markdown files with parsed frontmatter:

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `path` | TEXT | Relative path from vault root |
| `title` | TEXT | From frontmatter or filename |
| `tags` | TEXT[] | Combined frontmatter + inline tags |
| `aliases` | TEXT[] | From frontmatter |
| `frontmatter` | JSONB | Custom frontmatter fields |
| `body` | TEXT | Markdown without frontmatter |
| `raw_content` | TEXT | Original file content |
| `outgoing_links` | TEXT[] | Extracted [[wikilinks]] |
| `content_hash` | TEXT | SHA256 for change detection |

### vault_attachments

Stores binary files (images, PDFs, etc.):

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `path` | TEXT | Relative path from vault root |
| `mime_type` | TEXT | Detected content type |
| `data` | BYTEA | File content |
| `content_hash` | TEXT | SHA256 for change detection |

## Running as a Service

### macOS (launchd)

Create `~/Library/LaunchAgents/com.obsync-pg.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.obsync-pg</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/obsync-pg</string>
        <string>daemon</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>DB_PASSWORD</key>
        <string>your-password</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/obsync-pg.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/obsync-pg.err</string>
</dict>
</plist>
```

Load with: `launchctl load ~/Library/LaunchAgents/com.obsync-pg.plist`

### Linux (systemd)

Create `/etc/systemd/system/obsync-pg.service`:

```ini
[Unit]
Description=Obsidian Vault Sync Daemon
After=network.target

[Service]
Type=simple
User=your-username
Environment="DB_PASSWORD=your-password"
ExecStart=/usr/local/bin/obsync-pg daemon
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable with:
```bash
sudo systemctl enable obsync-pg
sudo systemctl start obsync-pg
```

### Windows (Task Scheduler)

1. Open Task Scheduler
2. Create Basic Task
3. Set trigger to "When the computer starts"
4. Set action to start `obsync-pg.exe daemon`
5. In the action settings, add the working directory and ensure environment variables are set

## Multi-Device Sync

When setting up a new device:

1. Install obsync-pg and create config file
2. Run `obsync-pg pull` to download all files from the database
3. Start the daemon: `obsync-pg daemon`

The pull command only downloads files that don't exist locally or have different content (based on hash).

## Troubleshooting

### Connection refused
- Verify database host/port are correct
- Check firewall rules allow PostgreSQL connections
- Ensure `sslmode` matches your server configuration

### Permission denied
- Verify database user has permissions on the tables
- Check the vault path is readable

### Files not syncing
- Check `ignore_patterns` aren't matching your files
- Run with `-v` flag for debug output
- Verify file watcher is working: changes should appear in logs

### Large vault initial sync is slow
- This is normal for first sync with many files
- Progress bar shows current status
- Subsequent syncs are incremental and fast

## Development

```bash
# Run tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt

# Build for all platforms
make build-all
```

## License

MIT
