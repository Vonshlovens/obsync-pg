# Troubleshooting Guide

This guide covers common issues and their solutions.

## Connection Issues

### "connection refused"

**Symptoms:**
```
failed to connect to database: failed to ping database: dial tcp: connection refused
```

**Solutions:**

1. **Verify host and port:**
   ```bash
   # Test connection manually
   psql "postgres://user:password@host:port/database?sslmode=require"
   ```

2. **Check if PostgreSQL is running** (self-hosted):
   ```bash
   sudo systemctl status postgresql
   ```

3. **For Supabase:** Ensure you're using the correct host from Project Settings → Database

4. **Firewall:** Ensure port 5432 is open

### "password authentication failed"

**Symptoms:**
```
failed to connect to database: password authentication failed for user "postgres"
```

**Solutions:**

1. **Check environment variable:**
   ```bash
   echo $DB_PASSWORD
   # Should show your password (or at least not be empty)
   ```

2. **Verify no extra whitespace:**
   ```bash
   # Bad:
   export DB_PASSWORD=' password '

   # Good:
   export DB_PASSWORD='password'
   ```

3. **For Supabase:** Reset password in Dashboard → Project Settings → Database

### "SSL required" or SSL errors

**Symptoms:**
```
server does not support SSL, but SSL was required
```

**Solutions:**

1. **For local development without SSL:**
   ```yaml
   database:
     sslmode: "disable"
   ```

2. **For Supabase:** Always use `sslmode: "require"`

### Timeout connecting

**Symptoms:**
```
context deadline exceeded
```

**Solutions:**

1. **Check network connectivity:**
   ```bash
   ping db.xxx.supabase.co
   ```

2. **For Supabase:** Check if your IP needs allowlisting (Project Settings → Database → Network)

3. **VPN/proxy:** Some networks block PostgreSQL port 5432

## Migration Issues

### "relation already exists"

**Symptoms:**
```
ERROR: relation "vault_notes" already exists
```

**Solutions:**

This happens if you run migrations twice or if tables exist from a previous setup.

1. **Check goose version table:**
   ```sql
   SELECT * FROM my_vault.goose_db_version;
   ```

2. **If starting fresh, drop the schema:**
   ```sql
   DROP SCHEMA my_vault CASCADE;
   ```
   Then run `obsync-pg migrate` again.

### "permission denied for schema"

**Symptoms:**
```
ERROR: permission denied for schema my_vault
```

**Solutions:**

1. **Grant permissions:**
   ```sql
   GRANT ALL ON SCHEMA my_vault TO postgres;
   GRANT ALL ON ALL TABLES IN SCHEMA my_vault TO postgres;
   ```

2. **For Supabase:** The default `postgres` user should have all permissions. Check you're using the correct user.

### "cannot find migrations directory"

**Symptoms:**
```
failed to run migrations: no migration files found
```

**Solutions:**

1. **Specify the migrations directory:**
   ```bash
   obsync-pg migrate --dir /path/to/obsync-pg/migrations
   ```

2. **Run from the project directory:**
   ```bash
   cd /path/to/obsync-pg
   obsync-pg migrate
   ```

## Sync Issues

### Files not syncing

**Symptoms:** Files are created/modified but don't appear in the database.

**Solutions:**

1. **Check if file matches ignore pattern:**
   ```yaml
   ignore_patterns:
     - ".obsidian/**"  # Is your file in .obsidian?
   ```

2. **Check daemon is running:**
   ```bash
   ps aux | grep obsync-pg
   ```

3. **Run with verbose logging:**
   ```bash
   obsync-pg daemon -v
   ```
   Look for "file event" and "sync" messages.

4. **Check the debounce delay:** Changes aren't synced instantly. Wait 2+ seconds after the last edit.

### Large files not syncing

**Symptoms:** Some attachments don't appear in database.

**Solutions:**

Check file size against `max_binary_size_mb` setting:
```yaml
sync:
  max_binary_size_mb: 50  # Files > 50MB are skipped
```

Logs will show:
```
level=WARN msg="attachment too large, skipping" path=large-file.pdf size_mb=75 max_mb=50
```

### "UNIQUE constraint failed" or duplicate errors

**Symptoms:**
```
ERROR: duplicate key value violates unique constraint "vault_notes_path_key"
```

**Solutions:**

This shouldn't happen in normal operation. If it does:

1. **Check for path normalization issues** (Windows backslashes vs forward slashes)

2. **Force full resync:**
   ```bash
   # Delete local state
   rm ~/.config/obsync-pg/state-*.json

   # Resync
   obsync-pg sync
   ```

### Changes not appearing on other devices

**Solutions:**

1. **Verify same schema:**
   ```bash
   # On each device
   obsync-pg status
   # Check "Schema:" matches
   ```

2. **Check daemon is running on all devices**

3. **Manual sync test:**
   ```bash
   # Device A: Make a change, then
   obsync-pg sync

   # Device B:
   obsync-pg pull
   ```

## Performance Issues

### Slow initial sync

**Expected behavior:** First sync of a large vault (1000+ files) can take several minutes.

**Optimizations:**

1. **Increase batch size:**
   ```yaml
   sync:
     batch_size: 200  # Default is 100
   ```

2. **Reduce binary size limit:**
   ```yaml
   sync:
     max_binary_size_mb: 20  # Skip large attachments
   ```

3. **Use include patterns to sync only essential folders:**
   ```yaml
   include_patterns:
     - "notes/**"
     - "journal/**"
   ```

### High CPU usage

**Symptoms:** `obsync-pg` using excessive CPU.

**Solutions:**

1. **Increase debounce time:**
   ```yaml
   sync:
     debounce_ms: 5000  # 5 seconds instead of 2
   ```

2. **Add noisy directories to ignore:**
   ```yaml
   ignore_patterns:
     - ".obsidian/**"
     - "**/node_modules/**"
     - "**/.git/**"
   ```

### Memory issues

**Symptoms:** Out of memory errors or high RAM usage.

**Solutions:**

1. **Reduce batch size:**
   ```yaml
   sync:
     batch_size: 50
   ```

2. **Lower binary size limit:**
   ```yaml
   sync:
     max_binary_size_mb: 10
   ```

## Parsing Issues

### Frontmatter not parsed correctly

**Symptoms:** Title, tags, or dates not appearing in database.

**Solutions:**

1. **Check YAML syntax:**
   ```yaml
   ---
   title: My Note
   tags:
     - tag1
     - tag2
   ---
   ```

2. **Common mistakes:**
   ```yaml
   # Bad: Missing quotes around colons
   title: Note: Part 1

   # Good:
   title: "Note: Part 1"
   ```

3. **Date formats supported:**
   - `2024-01-15`
   - `2024-01-15T10:30:00`
   - `2024-01-15 10:30:00`
   - ISO 8601 variants

### Wikilinks not extracted

**Symptoms:** `outgoing_links` array is empty.

**Solutions:**

Check link format:
```markdown
# Supported:
[[Page Name]]
[[Page Name|Display Text]]
[[folder/Page Name]]

# Not supported (yet):
![[embedded.png]]  # Embeds are different from links
```

### Tags not extracted

**Symptoms:** Tags from frontmatter or inline not appearing.

**Solutions:**

1. **Frontmatter tags:** Can be string or array
   ```yaml
   tags: single-tag        # Works
   tags: [tag1, tag2]      # Works
   tags:                   # Works
     - tag1
     - tag2
   ```

2. **Inline tags:** Must start with letter
   ```markdown
   #valid-tag      # Works
   #123            # Not detected (starts with number)
   #tag_name       # Works
   ```

## Log Analysis

### Enable verbose logging

```bash
obsync-pg daemon -v
```

### Key log messages

**Successful sync:**
```
level=INFO msg="file synced" path=notes/my-note.md hash=abc123
```

**File ignored:**
```
level=DEBUG msg="file ignored" path=.obsidian/workspace.json pattern=".obsidian/**"
```

**Connection issues:**
```
level=ERROR msg="sync failed" path=note.md error="connection refused"
```

**Retry:**
```
level=WARN msg="retry failed" path=note.md attempt=2 error="timeout"
```

## Getting Help

If you're still stuck:

1. **Check existing issues:** [GitHub Issues](https://github.com/deveric/obsync-pg/issues)

2. **Gather diagnostic info:**
   ```bash
   obsync-pg --version
   obsync-pg status
   obsync-pg daemon -v 2>&1 | head -100
   ```

3. **Open a new issue** with:
   - OS and version
   - PostgreSQL provider (Supabase, self-hosted, etc.)
   - Error messages
   - Relevant config (redact passwords!)
   - Steps to reproduce
