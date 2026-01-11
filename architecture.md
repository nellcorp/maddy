# Maddy Mail Server - Architecture

This document describes the architecture of this Maddy Mail Server fork, focusing on the additions made after forking from upstream (commit `cbeadf1`).

## Project Overview

Maddy is a composable, all-in-one email server written in Go that replaces Postfix, Dovecot, OpenDKIM, OpenSPF, and OpenDMARC with a single daemon. It supports SMTP (inbound/outbound), IMAP, and email security protocols (DKIM, SPF, DMARC).

**Fork Purpose:** Add REST API for user and mailbox management, enabling programmatic provisioning without CLI access.

## High-Level Architecture

```
                                    +------------------+
                                    |   REST API       |
                                    |   (Port 8080)    |
                                    |   [FORK ADDITION]|
                                    +--------+---------+
                                             |
        +------------------------------------+------------------------------------+
        |                                    |                                    |
        v                                    v                                    v
+-------+--------+                  +--------+--------+                  +--------+--------+
|  SMTP Inbound  |                  |   Submission    |                  |      IMAP       |
|   (Port 25)    |                  |   (Port 587)    |                  |   (Port 143)    |
+-------+--------+                  +--------+--------+                  +--------+--------+
        |                                    |                                    |
        +------------------------------------+------------------------------------+
                                             |
                                    +--------v--------+
                                    |  Message        |
                                    |  Pipeline       |
                                    +--------+--------+
                                             |
                    +------------------------+------------------------+
                    |                        |                        |
           +--------v--------+      +--------v--------+      +--------v--------+
           |   Credentials   |      |   Mailboxes     |      |  Message Blobs  |
           |   (PostgreSQL)  |      |   (PostgreSQL)  |      |      (S3)       |
           +-----------------+      +-----------------+      +-----------------+
```

## Upstream Components (Brief)

### Protocol Endpoints

| Endpoint | Port | Purpose |
|----------|------|---------|
| SMTP | 25 | Inbound mail reception (MX) |
| Submission | 587 | Authenticated outbound mail |
| IMAP | 143 | Mailbox access for clients |

Key files:
- `internal/endpoint/smtp/smtp.go` - SMTP server
- `internal/endpoint/imap/imap.go` - IMAP server

### Message Pipeline

Routes messages through checks, modifiers, and delivery targets based on configuration rules.

Key file: `internal/msgpipeline/msgpipeline.go`

### Authentication System

Pluggable authentication providers:
- `pass_table` - Database-backed password storage (used by this fork)
- `ldap`, `pam`, `shadow` - Alternative providers

Key interface: `module.PlainUserDB` in `framework/module/auth.go`

### Storage Layer

SQL-based IMAP storage with pluggable blob backends.

Key file: `internal/storage/imapsql/imapsql.go`

---

## Fork Additions (Detailed)

### 1. REST API Architecture

The REST API enables programmatic user and mailbox management.

```
+------------------+     +------------------+     +------------------+
|   HTTP Client    | --> |   Echo Server    | --> |   Middleware     |
|                  |     |   (Port 8080)    |     |   Stack          |
+------------------+     +------------------+     +--------+---------+
                                                          |
                    +-------------------------------------+
                    |
                    v
+-------------------+-------------------+-------------------+
|                   |                   |                   |
v                   v                   v                   v
Logger          Recovery            CORS               Rate Limiter
                                                      (3 req/sec)
|                   |                   |                   |
+-------------------+-------------------+-------------------+
                    |
                    v
+-------------------+-------------------+-------------------+
|                   |                   |                   |
v                   v                   v                   v
Security         CSRF               Request ID          Gzip
Headers         (optional)                              Compression
|                   |                   |                   |
+-------------------+-------------------+-------------------+
                    |
                    v
           +--------+--------+
           |   Basic Auth    |
           |   (Admin Only)  |
           +--------+--------+
                    |
                    v
           +--------+--------+
           |    Handlers     |
           |  (users.go,     |
           | imapAccounts.go)|
           +-----------------+
```

#### API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/` | Health check | No |
| GET | `/version` | Server version | No |
| POST | `/v1/users` | Create user | Yes |
| GET | `/v1/users` | List users (optional `?domain=` filter) | Yes |
| GET | `/v1/users/:id` | Get user | Yes |
| POST | `/v1/users/:id/password` | Update password | Yes |
| DELETE | `/v1/users/:id` | Delete user (optional `?delete_mailbox=true`) | Yes |
| POST | `/v1/users/:id/mailboxes` | Create mailbox | Yes |
| DELETE | `/v1/users/:id/mailboxes` | Delete mailbox | Yes |
| GET | `/v1/users/:id/quota` | Get user quota with mailbox breakdown | Yes |
| PUT | `/v1/users/:id/quota` | Set user quota override | Yes |
| GET | `/v1/domains/:domain/quota` | Get domain quota with user breakdown | Yes |
| PUT | `/v1/domains/:domain/quota` | Set domain quota limit | Yes |

#### Request/Response Models

```go
// Create user with optional mailbox provisioning
type CreateUserDto struct {
    Username        string `json:"username" validate:"email"`
    Password        string `json:"password,omitempty"`
    CreateMailboxes bool   `json:"createMailboxes"`
}

// User representation
type User struct {
    Username string `json:"username" validate:"email"`
    Password string `json:"password,omitempty"`
}

// Password update
type Password struct {
    Password string `json:"password,omitempty" validate:"required"`
}

// Quota responses
type UserQuotaResponse struct {
    Username    string         `json:"username"`
    UsedBytes   int64          `json:"usedBytes"`
    QuotaBytes  int64          `json:"quotaBytes"`  // 0 = unlimited
    QuotaSource string         `json:"quotaSource"` // "user", "domain", "none"
    Mailboxes   []MailboxUsage `json:"mailboxes"`
}

type DomainQuotaResponse struct {
    Domain     string      `json:"domain"`
    UsedBytes  int64       `json:"usedBytes"`
    QuotaBytes int64       `json:"quotaBytes"`
    UserCount  int64       `json:"userCount"`
    Users      []UserUsage `json:"users"`
}

type SetQuotaRequest struct {
    QuotaBytes int64 `json:"quotaBytes" validate:"gte=0"`
}
```

#### Key Files

| File | Purpose |
|------|---------|
| `api.go` | Route registration, server startup, DB initialization |
| `users.go` | User CRUD handlers and business logic |
| `imapAccounts.go` | Mailbox create/delete handlers |
| `quota.go` | Quota management handlers (get/set user and domain quotas) |
| `util.go` | DB access helpers, mailbox config extraction |
| `internal/rest/model/user.go` | Request/response DTOs |
| `internal/rest/model/quota.go` | Quota request/response DTOs |
| `internal/storage/imapsql/quota.go` | Quota enforcement (CheckQuota method) |
| `internal/rest/util/server/server.go` | Echo server setup with middleware |
| `internal/rest/util/server/secure.go` | Security headers, CORS configuration |
| `internal/rest/util/middleware/basic_auth/` | Admin authentication |

#### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `ENABLE_API` | Set to `true` to enable REST API | Yes |
| `ADMIN_EMAIL` | Admin username for Basic Auth | Yes (if API enabled) |
| `ADMIN_PASSWORD` | Admin password for Basic Auth | Yes (if API enabled) |
| `REQUESTS_PER_SECOND` | Rate limit (default: 3) | No |
| `CORS_ALLOW_ORIGINS` | Comma-separated allowed origins | No |
| `CONTENT_SECURITY_POLICY` | CSP header value | No |
| `FQDN` | Domain for CSRF cookie | No |
| `USE_COOKIES` | Enable CSRF (default: false) | No |

### 2. User/Mailbox Decoupling

Users and mailboxes have independent lifecycles, allowing flexible provisioning workflows.

```
WORKFLOW A: Create user without mailbox
+-------------------+
| POST /v1/users    |
| createMailboxes:  |
| false             |
+--------+----------+
         |
         v
+--------+----------+
| User created in   |
| credentials DB    |
+-------------------+


WORKFLOW B: Create user with mailbox
+-------------------+
| POST /v1/users    |
| createMailboxes:  |
| true              |
+--------+----------+
         |
         v
+--------+----------+     +-------------------+
| User created in   | --> | Mailbox created   |
| credentials DB    |     | with SPECIAL-USE  |
+-------------------+     | folders           |
                          +-------------------+


WORKFLOW C: Provision mailbox later
+-------------------+     +-------------------+
| POST /v1/users/   | --> | Mailbox created   |
| :id/mailboxes     |     | Sent, Trash,      |
+-------------------+     | Junk, Drafts,     |
                          | Archive           |
                          +-------------------+


WORKFLOW D: Delete user, keep mailbox
+-------------------+
| DELETE /v1/users/ |
| :id               |
+--------+----------+
         |
         v
+--------+----------+
| User removed from |
| credentials DB    |
| (mailbox remains) |
+-------------------+
```

#### Mailbox Creation Details

When a mailbox is created, these IMAP SPECIAL-USE folders are provisioned:

| Folder | SPECIAL-USE Attribute |
|--------|----------------------|
| Sent | `\Sent` |
| Trash | `\Trash` |
| Spam/Junk | `\Junk` |
| Drafts | `\Drafts` |
| Archive | `\Archive` |

Folder names are read from the `local_mailboxes` configuration block.

### 3. Quota Management

The quota system enables storage limits at domain and user levels with SMTP delivery enforcement.

```
QUOTA HIERARCHY:

+-------------------+
|   Domain Quota    |  <- Set via PUT /v1/domains/:domain/quota
|   (domain_quotas  |
|    table)         |
+--------+----------+
         |
         | inherited by all users unless overridden
         v
+--------+----------+
|   User Override   |  <- Set via PUT /v1/users/:id/quota
|   (users.         |
|   msgsizelimit)   |
+--------+----------+
         |
         | effective quota = user override > domain quota > unlimited
         v
+--------+----------+
|   SMTP Delivery   |  <- Checked in AddRcpt() and Body()
|   Enforcement     |     Returns 552 "Mailbox quota exceeded"
+-------------------+
```

**Quota Calculation:**
- Storage usage = `SUM(msgs.bodylen)` per user
- Checked before accepting recipient (AddRcpt)
- Checked with actual message size (Body)

**Response Examples:**

```json
// GET /v1/users/user@example.com/quota
{
  "username": "user@example.com",
  "usedBytes": 1234567,
  "quotaBytes": 10485760,
  "quotaSource": "domain",
  "mailboxes": [
    {"name": "INBOX", "messageCount": 50, "usedBytes": 800000},
    {"name": "Sent", "messageCount": 30, "usedBytes": 400000}
  ]
}

// GET /v1/domains/example.com/quota
{
  "domain": "example.com",
  "usedBytes": 5000000,
  "quotaBytes": 10485760,
  "userCount": 3,
  "users": [
    {"username": "alice@example.com", "usedBytes": 2000000},
    {"username": "bob@example.com", "usedBytes": 1500000, "quotaOverride": 5242880}
  ]
}
```

### 4. DNS Provider Support

Added build support for AWS Route53 and Cloudflare DNS providers for ACME certificate automation.

**Dockerfile change:**
```dockerfile
RUN ./build.sh --builddir /tmp/maddy-build \
    --destdir /tmp/maddy-install \
    --tags 'libdns_route53 libdns_cloudflare' \
    build install
```

These providers enable automatic DNS-01 challenges for Let's Encrypt certificates.

---

## Storage Architecture

```
+------------------+     +------------------+     +------------------+
|    REST API /    |     |     maddy.conf   |     |   Environment    |
|    Handlers      |     |                  |     |   Variables      |
+--------+---------+     +--------+---------+     +--------+---------+
         |                        |                        |
         +------------------------+------------------------+
                                  |
                                  v
                    +-------------+-------------+
                    |     storage.imapsql       |
                    |     (local_mailboxes)     |
                    +-------------+-------------+
                                  |
              +-------------------+-------------------+
              |                                       |
              v                                       v
+-------------+-------------+           +-------------+-------------+
|   PostgreSQL Database     |           |   S3 Blob Storage         |
|                           |           |                           |
|  - Message metadata       |           |  - Message bodies         |
|  - Mailbox structure      |           |  - Attachments            |
|  - Flags, UIDs            |           |                           |
|                           |           |                           |
|  DSN: host={DB_HOST}      |           |  endpoint: S3 URL         |
|       port={DB_PORT}      |           |  bucket: bucket name      |
|       dbname={MAILBOXES_  |           |  access_key/secret_key    |
|              DB_NAME}     |           |                           |
+---------------------------+           +---------------------------+

+---------------------------+
|  Credentials Database     |
|  (table.sql_query)        |
|                           |
|  - username (PK)          |
|  - password (bcrypt)      |
|                           |
|  DSN: host={DB_HOST}      |
|       port={DB_PORT}      |
|       dbname={DB_NAME}    |
+---------------------------+
```

### Database Tables

**Credentials Database (managed by `table.sql_query`):**
```sql
CREATE TABLE credentials (
    username varchar NOT NULL PRIMARY KEY,
    password varchar NOT NULL  -- bcrypt hash
);
```

**Mailboxes Database (managed by `go-imap-sql`):**

The imapsql module creates and manages three key tables:

**1. users** - IMAP account records
```
+---------------+--------------+------+-----------------------------------+
| Column        | Type         | Null | Description                       |
+---------------+--------------+------+-----------------------------------+
| id            | int8         | NO   | Primary key (auto-increment)      |
| username      | varchar(255) | NO   | Email address (unique)            |
| msgsizelimit  | int4         | YES  | Per-user message size limit       |
| inboxid       | int8         | YES  | Reference to INBOX mailbox        |
+---------------+--------------+------+-----------------------------------+
```

**2. mboxes** - Mailbox/folder records
```
+---------------+--------------+------+-----------------------------------+
| Column        | Type         | Null | Description                       |
+---------------+--------------+------+-----------------------------------+
| id            | int8         | NO   | Primary key (auto-increment)      |
| uid           | int4         | NO   | FK to users.id                    |
| name          | varchar(255) | NO   | Folder name (INBOX, Sent, etc.)   |
| sub           | int4         | NO   | Subscribed flag (default: 1)      |
| mark          | int4         | NO   | Internal marker                   |
| msgsizelimit  | int4         | YES  | Per-mailbox size limit            |
| uidnext       | int4         | NO   | Next UID to assign                |
| uidvalidity   | int8         | NO   | IMAP UIDVALIDITY value            |
| specialuse    | varchar(255) | YES  | SPECIAL-USE attribute (\Sent etc) |
| msgscount     | int4         | NO   | Message count in mailbox          |
+---------------+--------------+------+-----------------------------------+
```

**3. msgs** - Message metadata
```
+---------------+--------------+------+-----------------------------------+
| Column        | Type         | Null | Description                       |
+---------------+--------------+------+-----------------------------------+
| mboxid        | int8         | NO   | FK to mboxes.id                   |
| msgid         | int8         | NO   | Message UID within mailbox        |
| date          | int8         | NO   | Message date (Unix timestamp)     |
| bodylen       | int4         | NO   | Message body size in bytes        |
| mark          | int4         | NO   | Internal marker (default: 0)      |
| bodystructure | bytea        | NO   | IMAP BODYSTRUCTURE (serialized)   |
| cachedheader  | bytea        | NO   | Cached headers (serialized)       |
| extbodykey    | varchar(255) | YES  | FK to extkeys.id (S3 blob key)    |
| seen          | int4         | NO   | \Seen flag (default: 0)           |
| compressalgo  | varchar(255) | YES  | Compression algorithm used        |
| recent        | int4         | NO   | \Recent flag (default: 1)         |
+---------------+--------------+------+-----------------------------------+
```

**4. domain_quotas** - Domain quota limits (fork addition)
```
+---------------+--------------+------+-----------------------------------+
| Column        | Type         | Null | Description                       |
+---------------+--------------+------+-----------------------------------+
| id            | int8         | NO   | Primary key (auto-increment)      |
| domain        | varchar(255) | NO   | Domain name (unique)              |
| quota_bytes   | int8         | NO   | Quota limit in bytes (0=unlimited)|
| created_at    | timestamp    | YES  | Creation timestamp                |
| updated_at    | timestamp    | YES  | Last update timestamp             |
+---------------+--------------+------+-----------------------------------+
```

**5. user_quotas** - User quota overrides (fork addition)
```
+---------------+--------------+------+-----------------------------------+
| Column        | Type         | Null | Description                       |
+---------------+--------------+------+-----------------------------------+
| id            | int8         | NO   | Primary key (auto-increment)      |
| username      | varchar(255) | NO   | Email address (unique)            |
| quota_bytes   | int8         | NO   | Quota limit in bytes (0=unlimited)|
| created_at    | timestamp    | YES  | Creation timestamp                |
| updated_at    | timestamp    | YES  | Last update timestamp             |
+---------------+--------------+------+-----------------------------------+
```

**Usage Notes:**
- `msgs.bodylen` is used to calculate storage usage (aggregated per user/mailbox)
- `user_quotas.quota_bytes` stores user-specific quota override (takes precedence over domain)
- `domain_quotas.quota_bytes` stores domain-wide quota (inherited by all users in domain)
- `users.msgsizelimit` is for per-message size limits (NOT total quota)
- `msgs.extbodykey` links to blob storage (S3) for actual message content
- `mboxes.msgscount` tracks message count per folder
- `mboxes.specialuse` stores IMAP SPECIAL-USE attributes (`\Sent`, `\Trash`, `\Junk`, `\Drafts`, `\Archive`)

---

## Directory Structure

### Fork-Added Directories

```
/
├── api.go                    # REST API initialization
├── users.go                  # User endpoint handlers
├── imapAccounts.go           # Mailbox endpoint handlers
├── quota.go                  # Quota management handlers
├── util.go                   # DB helpers, config extraction
│
├── internal/rest/
│   ├── model/
│   │   ├── user.go           # User request/response DTOs
│   │   └── quota.go          # Quota request/response DTOs
│   │
│   └── util/
│       ├── middleware/
│       │   └── basic_auth/
│       │       └── basic_auth.go  # Admin auth validator
│       │
│       └── server/
│           ├── server.go     # Echo server, middleware stack
│           ├── secure.go     # Security headers, CORS
│           ├── ratelimiter.go # Rate limiting
│           └── binding.go    # Custom validation
│
└── internal/storage/imapsql/
    └── quota.go              # CheckQuota method for enforcement
```

### Key Upstream Directories

```
/
├── cmd/maddy/                # Main executable
├── framework/
│   ├── module/               # Module interfaces
│   └── config/               # Configuration parsing
├── internal/
│   ├── endpoint/smtp/        # SMTP server
│   ├── endpoint/imap/        # IMAP server
│   ├── storage/imapsql/      # SQL storage backend
│   ├── auth/pass_table/      # Password authentication
│   └── msgpipeline/          # Message routing
└── maddy.go                  # Server entry point
```

---

## Configuration

### Enabling REST API

Set environment variables before starting:

```bash
export ENABLE_API=true
export ADMIN_EMAIL=admin@example.com
export ADMIN_PASSWORD=secure-password
```

### Database Environment Variables

| Variable | Description |
|----------|-------------|
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port |
| `DB_NAME` | Credentials database name |
| `DB_USER` | Database user |
| `DB_PASSWORD` | Database password |
| `MAILBOXES_DB_NAME` | Mailboxes database name |

---

## Reference Configuration

Below is an annotated example configuration used in production:

```
# Base variables
$(hostname) = example.org
$(primary_domain) = example.org
$(local_domains) = $(primary_domain)

# Credentials stored in PostgreSQL
table.sql_query credentials {
    driver postgres
    dsn "host={env:DB_HOST} port={env:DB_PORT} dbname={env:DB_NAME} user={env:DB_USER} password={env:DB_PASSWORD} sslmode=disable"
    init "CREATE TABLE IF NOT EXISTS credentials (...)"
    lookup "SELECT password FROM credentials WHERE username = $1"
    list "SELECT username FROM credentials"
    add "INSERT INTO credentials (username, password) VALUES ($1, $2)"
    del "DELETE from credentials where username=$1"
    set "UPDATE credentials set password=$2 where username=$1"
}

# Password authentication using the credentials table
auth.pass_table local_authdb {
    table &credentials
}

# IMAP storage with PostgreSQL metadata and S3 blob storage
storage.imapsql local_mailboxes {
    driver postgres
    dsn "host={env:DB_HOST} port={env:DB_PORT} dbname={env:MAILBOXES_DB_NAME} ..."

    msg_store s3 {
        creds access_key
        endpoint "s3-endpoint"
        access_key "key"
        secret_key "secret"
        bucket "bucket-name"
    }

    # Mailbox names used by REST API
    sent_mailbox Sent
    trash_mailbox Trash
    junk_mailbox Spam
    drafts_mailbox Drafts
    archive_mailbox Archive
}

# SMTP inbound (port 25)
smtp tcp://0.0.0.0:25 {
    # ... routing rules
}

# Submission (port 587)
submission tcp://0.0.0.0:587 {
    auth &local_authdb
    # ... routing rules with DKIM signing
}

# IMAP (port 143)
imap tcp://0.0.0.0:143 {
    auth &local_authdb
    storage &local_mailboxes
}

# Outbound relay
target.smtp outbound_delivery_relay {
    targets tcp://relay-host:25
    auth plain "user" "password"
}
```

**Key Points:**
- `local_authdb` and `local_mailboxes` are the module names the REST API looks for
- The REST API reads mailbox folder names from the `storage.imapsql` config
- S3 blob storage enables scalable message storage
- Relay delivery is used for outbound mail through an external SMTP server
