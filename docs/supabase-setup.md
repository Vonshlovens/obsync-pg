# Supabase Setup Guide

This guide walks through setting up Supabase as the PostgreSQL backend for Obsync-PG.

## Why Supabase?

- **Free tier**: 500MB database, perfect for most vaults
- **Managed PostgreSQL**: No server maintenance
- **Built-in SSL**: Secure connections out of the box
- **Dashboard**: SQL Editor for querying your notes

## Step 1: Create a Supabase Project

1. Go to [supabase.com](https://supabase.com) and sign up/login
2. Click **New Project**
3. Fill in:
   - **Name**: e.g., "obsidian-sync"
   - **Database Password**: Generate a strong password and **save it somewhere safe**
   - **Region**: Choose one close to you
4. Click **Create new project**
5. Wait for the project to be provisioned (1-2 minutes)

## Step 2: Get Connection Details

1. In your Supabase dashboard, go to **Project Settings** (gear icon)
2. Click **Database** in the sidebar
3. Find the **Connection string** section

You'll need these values:

| Setting | Where to find | Example |
|---------|---------------|---------|
| Host | Connection string URI | `db.abcdefghijklmnop.supabase.co` |
| Port | Connection string | `5432` (direct) or `6543` (pooler) |
| User | Usually | `postgres` |
| Password | You set this during project creation | `your-password` |
| Database | Usually | `postgres` |

### Direct vs Pooled Connection

Supabase offers two connection modes:

- **Direct connection** (port 5432): Best for long-running processes like the daemon
- **Connection pooler** (port 6543): Better for serverless/short connections

**For Obsync-PG, use the direct connection (port 5432).**

## Step 3: Configure Obsync-PG

Run the setup wizard:

```bash
obsync-pg init
```

Enter your Supabase details:

```
=== Obsync-PG Setup ===

Obsidian vault path: /path/to/your/vault

Database Configuration:
  Host: db.abcdefghijklmnop.supabase.co
  Port [5432]: 5432
  User: postgres
  Password: your-supabase-db-password
  Database name: postgres
  Schema name [your_vault_name]:
  SSL mode [require]: require
```

## Step 4: Set the Password Environment Variable

```bash
# Linux/macOS
export DB_PASSWORD='your-supabase-db-password'

# Windows PowerShell
$env:DB_PASSWORD = "your-supabase-db-password"
```

## Step 5: Run Migrations

```bash
obsync-pg migrate
```

This creates:
- Your vault's schema (e.g., `my_obsidian_vault`)
- `vault_notes` table
- `vault_attachments` table
- Required indexes

## Step 6: Verify in Supabase Dashboard

1. Go to your Supabase project
2. Click **SQL Editor** in the sidebar
3. Run this query to see your schema:

```sql
-- List all schemas
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast');

-- List tables in your vault's schema
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'your_vault_schema_name';
```

## Step 7: Start Syncing

```bash
# One-time sync
obsync-pg sync

# Or start the daemon for continuous sync
obsync-pg daemon
```

## Querying Your Notes in Supabase

Once synced, you can query your notes directly in the SQL Editor:

```sql
-- Set the search path to your vault's schema
SET search_path TO my_obsidian_vault, public;

-- Count all notes
SELECT COUNT(*) FROM vault_notes;

-- Find notes with a specific tag
SELECT path, title
FROM vault_notes
WHERE 'project' = ANY(tags);

-- Search note content
SELECT path, title, LEFT(body, 200) as preview
FROM vault_notes
WHERE body ILIKE '%search term%';

-- Find all outgoing links from a note
SELECT path, outgoing_links
FROM vault_notes
WHERE 'Some Page' = ANY(outgoing_links);

-- Get notes modified in the last week
SELECT path, title, modified_at
FROM vault_notes
WHERE modified_at > NOW() - INTERVAL '7 days'
ORDER BY modified_at DESC;

-- Query custom frontmatter fields
SELECT path, title, frontmatter->>'status' as status
FROM vault_notes
WHERE frontmatter->>'status' = 'in-progress';
```

## Multiple Vaults on One Supabase Project

Each vault gets its own schema, so you can sync multiple vaults to one Supabase project:

```
postgres (database)
├── my_personal_vault (schema)
│   ├── vault_notes
│   └── vault_attachments
├── work_notes (schema)
│   ├── vault_notes
│   └── vault_attachments
└── public (schema - Supabase default)
```

Just run `obsync-pg init` for each vault with a different schema name.

## Supabase Free Tier Limits

| Resource | Free Tier Limit |
|----------|-----------------|
| Database size | 500 MB |
| Bandwidth | 2 GB/month |
| API requests | Unlimited |

**Typical vault sizes:**
- 500 notes with frontmatter: ~5-10 MB
- 1000 notes: ~10-20 MB
- Including attachments varies widely

You can check your usage in Supabase Dashboard → Project Settings → Usage.

## Troubleshooting

### "connection refused" or timeout

- Check your host is correct (should be `db.xxxx.supabase.co`)
- Verify you're using port `5432` (not `6543`)
- Check if your IP needs to be allowlisted (Project Settings → Database → Network)

### "password authentication failed"

- Verify DB_PASSWORD environment variable is set
- Make sure there are no extra spaces or quotes
- Reset password in Supabase Dashboard if needed

### "permission denied for schema"

- The postgres user should have full permissions by default
- Try running: `GRANT ALL ON SCHEMA your_schema TO postgres;`

### Tables not visible in Supabase Table Editor

The Table Editor only shows the `public` schema by default. Your tables are in a custom schema. Use SQL Editor to query them, or create views in `public` schema if needed:

```sql
-- Create a view in public schema to see notes in Table Editor
CREATE VIEW public.my_vault_notes AS
SELECT * FROM my_vault_schema.vault_notes;
```

## Security Notes

- Never commit your password to git
- Use environment variables for secrets
- Supabase connections are encrypted with SSL by default
- Consider using a dedicated database user with limited permissions for production
