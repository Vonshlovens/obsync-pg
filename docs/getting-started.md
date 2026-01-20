# Getting Started with Obsync-PG

This guide will walk you through setting up Obsync-PG to sync your Obsidian vault to a PostgreSQL database.

## Prerequisites

- **Go 1.22+** installed ([download](https://go.dev/dl/))
- **PostgreSQL database** (Supabase, self-hosted, or any PostgreSQL provider)
- **Obsidian vault** on your local machine

## Step 1: Install Obsync-PG

### Option A: Build from source

```bash
# Clone the repository
git clone https://github.com/vonshlovens/obsync-pg.git
cd obsync-pg

# Build the binary
go build -o bin/obsync-pg ./cmd/obsync-pg

# Optionally, move to your PATH
# Linux/macOS:
sudo mv bin/obsync-pg /usr/local/bin/
# Windows: Add bin/ to your PATH or move obsync-pg.exe to a directory in PATH
```

### Option B: Install directly with Go

```bash
go install github.com/vonshlovens/obsync-pg/cmd/obsync-pg@latest
```

### Verify installation

```bash
obsync-pg --version
obsync-pg --help
```

## Step 2: Set Up Your Database

### Using Supabase (Recommended)

See [Supabase Setup Guide](supabase-setup.md) for detailed instructions.

**Quick version:**
1. Create a Supabase project at [supabase.com](https://supabase.com)
2. Go to Project Settings → Database
3. Copy the connection details (host, port, user, password, database name)

### Using Self-Hosted PostgreSQL

```bash
# Create a database
psql -U postgres -c "CREATE DATABASE obsidian_sync;"
```

## Step 3: Configure Obsync-PG

Run the interactive setup wizard:

```bash
obsync-pg init
```

You'll be prompted for:
- **Obsidian vault path**: Full path to your vault folder (e.g., `/Users/you/Documents/MyVault`)
- **Database host**: Your PostgreSQL host (e.g., `db.xxxx.supabase.co`)
- **Port**: Usually `5432` (or `6543` for Supabase pooler)
- **User**: Database user (e.g., `postgres`)
- **Password**: Database password
- **Database name**: Your database name (e.g., `postgres` for Supabase)
- **Schema name**: Press Enter to auto-derive from vault name, or specify custom
- **SSL mode**: Use `require` for Supabase

**Example session:**
```
=== Obsync-PG Setup ===

Obsidian vault path: /Users/eric/Documents/MyObsidianVault

Database Configuration:
  Host: db.abcdefgh.supabase.co
  Port [5432]: 5432
  User: postgres
  Password: your-password-here
  Database name: postgres
  Schema name [myobsidianvault]:
  SSL mode [require]:

Config file written to: /Users/eric/.config/obsync-pg/config.yaml
```

## Step 4: Set Environment Variable

For security, the password is read from an environment variable:

**Linux/macOS:**
```bash
export DB_PASSWORD='your-database-password'

# Add to ~/.bashrc or ~/.zshrc for persistence
echo "export DB_PASSWORD='your-database-password'" >> ~/.bashrc
```

**Windows (PowerShell):**
```powershell
$env:DB_PASSWORD = "your-database-password"

# For persistence, set via System Properties > Environment Variables
```

**Windows (Command Prompt):**
```cmd
set DB_PASSWORD=your-database-password
```

## Step 5: Run Database Migrations

Create the required tables in your database:

```bash
obsync-pg migrate
```

Expected output:
```
time=... level=INFO msg="connected to database" host=db.xxx.supabase.co database=postgres schema=myobsidianvault
time=... level=INFO msg="schema ready" schema=myobsidianvault
time=... level=INFO msg="migrations completed successfully" schema=myobsidianvault
Migrations completed successfully.
```

## Step 6: Verify Connection

Check that everything is connected properly:

```bash
obsync-pg status
```

Expected output:
```
=== Obsync-PG Status ===
Database Status: Connected
  Host: db.xxx.supabase.co
  Database: postgres
  Schema: myobsidianvault

Vault Path: /Users/eric/Documents/MyObsidianVault

Synced Files:
  Notes: 0
  Attachments: 0
```

## Step 7: Run Your First Sync

Perform a one-time full sync:

```bash
obsync-pg sync
```

You'll see a progress bar as files are scanned and synced:

```
Scanning files  100% |████████████████████████████████████████| (150/150)
Syncing files   100% |████████████████████████████████████████| (150/150)
time=... level=INFO msg="full reconciliation completed" synced=150 deleted=0 duration_s=12.5
Sync completed successfully.
```

## Step 8: Start the Daemon

For continuous syncing, run the daemon:

```bash
obsync-pg daemon
```

The daemon will:
1. Perform an initial full sync
2. Watch for file changes
3. Sync changes in real-time (with 2-second debouncing)

```
time=... level=INFO msg="performing initial sync"
time=... level=INFO msg="full reconciliation completed" synced=0 deleted=0 duration_s=1.2
time=... level=INFO msg="watcher started" path=/Users/eric/Documents/MyObsidianVault
Watching vault for changes. Press Ctrl+C to stop.
```

Now edit a file in Obsidian - you'll see it sync automatically!

## Next Steps

- **Run as a service**: See the [README](../README.md#running-as-a-service) for systemd/launchd setup
- **Multiple devices**: See [Multi-Device Setup](multi-device.md)
- **Configuration options**: See [Configuration Reference](configuration.md)
- **Troubleshooting**: See [Troubleshooting Guide](troubleshooting.md)

## Quick Reference

| Command | Description |
|---------|-------------|
| `obsync-pg init` | Interactive setup wizard |
| `obsync-pg migrate` | Create database tables |
| `obsync-pg status` | Check connection and counts |
| `obsync-pg sync` | One-time full sync |
| `obsync-pg daemon` | Start real-time sync |
| `obsync-pg pull` | Download from DB to local |
