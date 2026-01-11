# CLAUDE.md

Maddy Mail Server fork with REST API for user/mailbox management. See [architecture.md](./architecture.md) for detailed system design.

## Quick Reference

### Build
```bash
./build.sh build                              # Build binaries
./build.sh --tags 'libdns_route53' build      # With Route53 support
docker build -t maddy .                       # Docker build
```

### Test
```bash
go test ./...                                 # Run all tests
go test ./internal/rest/...                   # Test REST API only
```

### Run
```bash
# Required for REST API
export ENABLE_API=true
export ADMIN_EMAIL=admin@example.com
export ADMIN_PASSWORD=secure-password

# Database config
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=maddy
export DB_USER=maddy
export DB_PASSWORD=password
export MAILBOXES_DB_NAME=mailboxes

./build/maddy run
```

## Fork Context

**Fork point:** `cbeadf169c8dfc3a824c9b83ab605fd792a300b7`

**Changes made:**
1. REST API for user/mailbox management (Echo framework)
2. User/mailbox lifecycle decoupling
3. Route53/Cloudflare DNS provider support for ACME

**Key files added/modified:**
- `api.go` - REST API routes and initialization
- `users.go` - User CRUD handlers
- `imapAccounts.go` - Mailbox handlers
- `util.go` - DB access helpers
- `internal/rest/` - REST API utilities and models

## Development Patterns

### Adding REST Endpoints

1. Define route in `api.go`:
```go
func NewV1(e *echo.Echo) {
    // ...
    v1.GET("/new-endpoint", newHandler)
}
```

2. Create handler in appropriate file (`users.go`, `imapAccounts.go`, or new file):
```go
func newHandler(c echo.Context) error {
    // Use global vars: userDb, imapDb, mailboxes
    return c.JSON(http.StatusOK, result)
}
```

3. Add DTOs to `internal/rest/model/` if needed:
```go
type NewDto struct {
    Field string `json:"field" validate:"required"`
}
```

### Request Binding and Validation

```go
func handler(c echo.Context) error {
    r := model.SomeDto{}
    if err := c.Bind(&r); err != nil {
        return err  // Returns 400 with validation errors
    }
    // Validation runs automatically via CustomBinder
    // ...
}
```

### Error Handling

Return errors directly - Echo handles HTTP status codes:
```go
if err != nil {
    return err  // Returns 500 Internal Server Error
}
return c.NoContent(http.StatusOK)
```

For specific status codes:
```go
return c.NoContent(http.StatusNotFound)
return echo.NewHTTPError(http.StatusBadRequest, "message")
```

### Database Access

Global variables initialized in `api.go`:
- `userDb` (`module.PlainUserDB`) - User credentials operations
- `imapDb` (`module.ManageableStorage`) - Mailbox operations
- `mailboxes` (`Mailboxes`) - Folder names from config

```go
// User operations
userDb.ListUsers()
userDb.DeleteUser(username)
userDb.SetUserPassword(username, password)

// Mailbox operations
imapDb.CreateIMAPAcct(username)
imapDb.DeleteIMAPAcct(username)
imapDb.GetIMAPAcct(username)
```

## Code Organization

### Where to add new features

| Feature Type | Location |
|-------------|----------|
| New REST endpoint | `api.go` (route), `*.go` (handler) |
| New DTO/model | `internal/rest/model/` |
| New middleware | `internal/rest/util/middleware/` |
| Server config | `internal/rest/util/server/` |
| Module interface changes | `framework/module/` |

### Important module names (in config)

The REST API looks for these specific module names:
- `local_authdb` - Password authentication module
- `local_mailboxes` - IMAP storage module

### Database Schema (Mailboxes DB)

The imapsql module manages three key tables:

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `users` | IMAP accounts | `id`, `username`, `msgsizelimit`, `inboxid` |
| `mboxes` | Mailbox folders | `id`, `uid` (FK→users), `name`, `specialuse`, `msgscount` |
| `msgs` | Message metadata | `mboxid` (FK→mboxes), `msgid`, `bodylen`, `extbodykey`, `seen` |

**Useful for future features:**
- `msgs.bodylen` - Calculate storage usage/quotas (not yet implemented)
- `mboxes.msgscount` - Message count per folder
- `msgs.extbodykey` - Links to S3 blob storage

See [architecture.md](./architecture.md#database-tables) for full schema details.

## Testing

### Manual API Testing

```bash
# Health check
curl http://localhost:8080/

# Create user (with auth)
curl -X POST http://localhost:8080/v1/users \
  -u admin@example.com:password \
  -H "Content-Type: application/json" \
  -d '{"username":"user@example.com","password":"pass","createMailboxes":true}'

# List users
curl http://localhost:8080/v1/users \
  -u admin@example.com:password

# Delete user
curl -X DELETE http://localhost:8080/v1/users/user@example.com \
  -u admin@example.com:password
```

### Integration Tests

Place tests alongside handlers or in `internal/rest/` directory.

## Configuration

### REST API Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ENABLE_API` | Yes | - | Set to `true` to enable |
| `ADMIN_EMAIL` | Yes* | - | Admin username for Basic Auth |
| `ADMIN_PASSWORD` | Yes* | - | Admin password for Basic Auth |
| `REQUESTS_PER_SECOND` | No | 3 | Rate limit per IP |
| `CORS_ALLOW_ORIGINS` | No | `*` | Comma-separated origins |
| `CONTENT_SECURITY_POLICY` | No | - | CSP header value |

*Required when `ENABLE_API=true`

### Database Environment Variables

| Variable | Description |
|----------|-------------|
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port (usually 5432) |
| `DB_NAME` | Credentials database |
| `DB_USER` | Database user |
| `DB_PASSWORD` | Database password |
| `MAILBOXES_DB_NAME` | Mailbox metadata database |

## Common Tasks

### Add a new user property

1. Update DTO in `internal/rest/model/user.go`
2. Update handler in `users.go`
3. May require upstream module interface changes

### Change mailbox folder names

Update `maddy.conf`:
```
storage.imapsql local_mailboxes {
    sent_mailbox "Sent Messages"
    trash_mailbox "Deleted Items"
    # ...
}
```

### Add new authentication provider

This requires upstream module changes in `internal/auth/` and implementing `module.PlainUserDB` interface.

### Debug REST API

```bash
# Enable debug mode (set in api.go)
# Check server logs for request/response info

# Test rate limiting
for i in {1..10}; do curl http://localhost:8080/; done
```
