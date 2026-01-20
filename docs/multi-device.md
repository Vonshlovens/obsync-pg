# Multi-Device Setup

This guide explains how to set up Obsync-PG on multiple devices to keep your vault synchronized.

## How It Works

```
┌─────────────────┐         ┌─────────────────┐
│   Device A      │         │   Device B      │
│  (Desktop)      │         │  (Laptop)       │
│                 │         │                 │
│  Obsidian       │         │  Obsidian       │
│  Vault          │         │  Vault          │
│      │          │         │      │          │
│      ▼          │         │      ▼          │
│  obsync-pg      │         │  obsync-pg      │
│  daemon         │         │  daemon         │
└────────┬────────┘         └────────┬────────┘
         │                           │
         │     ┌─────────────┐       │
         └────►│  PostgreSQL │◄──────┘
               │  (Supabase) │
               │             │
               │  my_vault   │
               │  schema     │
               └─────────────┘
```

Each device runs its own daemon that syncs changes to the same database schema. The database acts as the central source of truth.

## Important Concepts

### Source of Truth

- The **database** is the central source of truth
- When you start the daemon on an existing vault, local files are synced UP to the database
- When you run `pull` on a new device, files are synced DOWN from the database

### Conflict Handling

Currently, **last-write-wins**. If you edit the same file on two devices simultaneously:
1. Device A saves → syncs to DB
2. Device B saves → overwrites DB with its version

**Best practice:** Don't edit the same file on multiple devices simultaneously. The debounce delay (2 seconds default) helps prevent issues during normal use.

## Setting Up a New Device

### Step 1: Install Obsync-PG

```bash
# Build from source
git clone https://github.com/vonshlovens/obsync-pg.git
cd obsync-pg
go build -o bin/obsync-pg ./cmd/obsync-pg
sudo mv bin/obsync-pg /usr/local/bin/

# Or install directly
go install github.com/vonshlovens/obsync-pg/cmd/obsync-pg@latest
```

### Step 2: Create Config File

Create the config directory:

```bash
# Linux/macOS
mkdir -p ~/.config/obsync-pg

# Windows (PowerShell)
New-Item -ItemType Directory -Force -Path "$env:APPDATA\obsync-pg"
```

Create `config.yaml` with the **same database settings** as your first device:

```yaml
vault_path: "/Users/newuser/Documents/MyVault"  # Local path on THIS device

database:
  host: "db.xxx.supabase.co"      # Same as Device A
  port: 5432                       # Same as Device A
  user: "postgres"                 # Same as Device A
  password: "${DB_PASSWORD}"       # Same as Device A
  database: "postgres"             # Same as Device A
  schema: "my_vault"               # MUST be the same as Device A
  sslmode: "require"
```

**Critical:** The `schema` must match exactly. If Device A auto-derived it from the vault name, you should explicitly set it on Device B to ensure they match.

### Step 3: Set Environment Variable

```bash
export DB_PASSWORD='your-database-password'
```

### Step 4: Create Vault Directory

```bash
# The vault directory should exist but can be empty
mkdir -p /Users/newuser/Documents/MyVault
```

### Step 5: Pull Files from Database

```bash
obsync-pg pull
```

This downloads all files from the database to your local vault:

```
time=... level=INFO msg="pulling files from database to local vault"
Pulling files  100% |████████████████████████████████████████| (150/150)
time=... level=INFO msg="pull completed" notes=145 attachments=5 duration_s=8.2
Pull completed successfully.
```

### Step 6: Open in Obsidian

1. Open Obsidian
2. Click "Open folder as vault"
3. Select your vault directory
4. Your notes are now available!

### Step 7: Start the Daemon

```bash
obsync-pg daemon
```

Now changes on this device will sync to the database, and you'll receive changes from other devices.

## Workflow: Adding a Third Device

1. Install obsync-pg
2. Copy config.yaml from another device (update `vault_path` for this device)
3. Set `DB_PASSWORD` environment variable
4. Create empty vault directory
5. Run `obsync-pg pull`
6. Open vault in Obsidian
7. Run `obsync-pg daemon`

## Running the Daemon Automatically

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

Load it:
```bash
launchctl load ~/Library/LaunchAgents/com.obsync-pg.plist
```

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

Enable and start:
```bash
sudo systemctl enable obsync-pg
sudo systemctl start obsync-pg
```

### Windows (Task Scheduler)

1. Open Task Scheduler
2. Create Basic Task → "Obsync-PG Daemon"
3. Trigger: "When the computer starts"
4. Action: Start a program
   - Program: `C:\path\to\obsync-pg.exe`
   - Arguments: `daemon`
5. Finish, then edit the task:
   - Check "Run whether user is logged on or not"
   - Check "Run with highest privileges"

## Checking Sync Status

On any device, run:

```bash
obsync-pg status
```

Compare the note/attachment counts across devices to verify sync:

**Device A:**
```
Synced Files:
  Notes: 145
  Attachments: 5
  Last Sync: 2024-01-15T10:30:00Z
```

**Device B:**
```
Synced Files:
  Notes: 145
  Attachments: 5
  Last Sync: 2024-01-15T10:32:15Z
```

## Manual Sync

If the daemon isn't running, you can manually sync:

```bash
# Push local changes to DB
obsync-pg sync

# Pull DB changes to local (overwrites local if different)
obsync-pg pull
```

## Troubleshooting Multi-Device

### Files missing on new device

1. Check the schema name matches exactly
2. Run `obsync-pg status` to verify connection
3. Try `obsync-pg pull` again

### Duplicate/conflicting changes

1. Stop daemons on all devices
2. Decide which device has the "correct" version
3. Run `obsync-pg sync` on that device
4. Run `obsync-pg pull` on other devices
5. Restart daemons

### Changes not appearing on other devices

1. Check daemon is running: `ps aux | grep obsync-pg`
2. Check for errors in logs
3. Verify network connectivity to database
4. Run `obsync-pg status` to check connection

### Schema mismatch

If devices are using different schemas, they won't see each other's data:

```bash
# Check current schema
obsync-pg status
# Output: Schema: my_vault

# Compare with other device's schema
# If different, update config.yaml to use the same schema
```
