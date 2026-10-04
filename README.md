# 🚀 Go Common (`github.com/kamlesh-Sahani/go-commons`)

> A production-ready Go toolkit and microservice for multi-project architectures. Use it as an importable Go package in your backends (`go get github.com/kamlesh-Sahani/go-commons`) or run it as a standalone file upload microservice.

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 📌 What is inside?

| Package | What it does | Example Use Case |
| :--- | :--- | :--- |
| **`table`** | Universal table engine (Postgres, MySQL, Mongo, Slices) | Pagination, search, column filters & streaming Excel/CSV export |
| **`response`** | Standard JSON responses | One-line `response.Success(c, "OK", data)` or `response.BadRequest(...)` |
| **`utils`** | Form validation, currency, dates, strings | `notblank` validator, cents-to-dollars math, ISO date parsing |
| **`sdk`** | Client for the hosted S3 upload service | Confirm an uploaded invoice or avatar in 2 lines |
| **`middleware`** | Production Gin middlewares | CORS, rate limiting (100 req/s), panic recovery, body size guard |
| **`crypto`** | Password hashing & encryption | Bcrypt password hashing & AES-256-GCM data encryption |
| **`queue`** | RabbitMQ manager & consumer workers | Publish JSON events and run resilient background workers |

---

## ⚡ Quick Install

In your backend project (e.g. `billing-management` or `inventory_and_accounting`):

```bash
go get github.com/kamlesh-Sahani/go-commons
```

---

## 📖 Feature Guide with Simple Examples

### 1. 📊 Universal Table Engine (`table`)

The `table` engine is an enterprise data table pipeline that powers dynamic data grids (React, Vue, Angular, mobile) with pagination, multi-field search, column filters, and zero-memory streaming exports. It is completely **database-agnostic** and works seamlessly with **PostgreSQL, MySQL, SQLite, MongoDB, and in-memory slices**.

```
Frontend (React / Vue / Mobile)
       │
       ▼  POST /api/v1/entities/list (JSON)
Gin Router
       │
       ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        table Package Pipeline                          │
│                                                                        │
│  1. Parse & Validate   ──► Bounds checks (limit ≤ 100, export ≤ 50k)   │
│  2. Auto-Scoping       ──► Injects TenantID & SoftDelete (deleted_at)   │
│  3. Filter Synthesis   ──► Builds multi-field search & column filters  │
│  4. Keyset Boundary    ──► Decodes (ID, SortField, SortValue) cursor    │
│  5. Query Execution    ──► Parallel count & find / aggregate query     │
│  6. Row Mapping        ──► Maps entity struct to rich UI TableCells    │
│  7. Cursor Generation  ──► Cached reflection / CursorProvider (O(1))   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
           ┌────────────────────────┴────────────────────────┐
           ▼                                                 ▼
[Standard JSON Response]                        [Streaming CSV / Excel]
(TableResponseData payload)                     (O(1) RAM via chunked transfer)
```

---

#### 🔄 Step-by-Step Table Execution Flow

Every table request follows a predictable, highly optimized 8-step lifecycle:

```
[HTTP Request Received]
        │
        ▼
Step 1: Request Validation (table/handler.go)
        • Rejects non-POST methods.
        • Validates and clamps pagination bounds (Limit: 1-100; Export: 1-50,000).
        • Parses JSON payload into a strongly-typed table.ListRequest.
        │
        ▼
Step 2: Scoping & Isolation (table/handler.go)
        • Tenant Scoping: Injects `tenant_id = c.TenantID` into the base filter.
        • Soft-Delete: Injects `deleted_at: { $exists: false }` or `deleted_at IS NULL`.
        │
        ▼
Step 3: Filter & Search Synthesis (table/filter.go & table/sql_builder.go)
        • Search: Compiles multi-field `OR` conditions (e.g. `name ILIKE '%term%' OR email ILIKE '%term%'`).
        • Column Filters: Translates operators (`EQUAL`, `NOT_EQUAL`, `IN`, `BETWEEN`, `GREATER_THAN`, `CONTAINS`).
        • Fast-Path Coercion: Checks date patterns (`looksLikeDate`) and hex ObjectIDs without allocating errors.
        │
        ▼
Step 4: Pagination & Keyset Boundaries (table/cursor.go)
        • Keyset Cursor Mode: Decodes base64 cursor token `{"i": "id", "f": "created_at", "v": "value"}`.
        • Injects boundary clause: `(sortField < val) OR (sortField = val AND id < cursorID)`.
        • Offset Mode: Computes `Skip = (Page - 1) * Limit`.
        │
        ▼
Step 5: Database Query Execution (table/handler.go or table/generic.go)
        • If Export: Streams rows directly from the database cursor into the HTTP response socket.
        • If Listing: Dispatches parallel goroutines (`sync.WaitGroup`):
            - Goroutine A: `CountDocuments` (with 3-second timeout fallback to prevent slow count locks).
            - Goroutine B: `Find` or `Aggregate` with `Limit + 1` (to detect `HasMore`).
        │
        ▼
Step 6: Keyset Cursor Token Extraction (table/cursor.go)
        • Reads ID and sort value from the last record.
        • Uses `sync.Map` reflection cache (or fast `CursorProvider` interface) to bypass reflection overhead.
        • Base64-URL-encodes next token for the client.
        │
        ▼
Step 7: UI Cell Mapping (table/cells.go)
        • Runs `cfg.RowMapper(item)` to transform model data into semantic UI cells (Links, Badges, Currency, Dates).
        │
        ▼
Step 8: Response Delivery
        • Returns HTTP 200 with standard `TableResponseData`.
```

---

#### 📥 The Request & Response Contract

##### What the Frontend Sends (`POST` JSON body)
```json
{
  "page": 1,
  "limit": 20,
  "sortBy": "created_at",
  "order": "desc",
  "search": "john",
  "paginationType": "cursor",
  "cursor": "eyJpIjoiNWY4Yj...iwiZiI6ImNyZWF0ZWRfYXQiLCJ2IjoiMjAyNi0xMC0wNCJ9",
  "filters": {
    "status": {
      "operator": "EQUAL",
      "value": ["ACTIVE"]
    },
    "revenue": {
      "operator": "GREATER_OR_EQUAL",
      "value": ["1000"]
    }
  },
  "export": {
    "export": false,
    "format": "CSV"
  }
}
```

##### What the Backend Returns (`TableResponseData`)
```json
{
  "success": true,
  "message": "Companies fetched successfully",
  "data": {
    "columns": [
      { "id": "name", "title": "Company Name", "sortable": true },
      { "id": "revenue", "title": "Revenue", "sortable": true },
      { "id": "status", "title": "Status", "sortable": false }
    ],
    "rows": [
      {
        "name": [{ "type": "TEXT", "title": "Acme Corp", "isBold": true }],
        "revenue": [{ "type": "CURRENCY", "title": "USD 12000.00" }],
        "status": [{ "type": "TAG", "title": "Active", "bgColor": "#dcfce7", "textColor": "#15803d" }]
      }
    ],
    "isPaginated": true,
    "pageNumber": 1,
    "totalPages": 5,
    "pageSize": 20,
    "totalRows": 95,
    "nextCursor": "eyJpIjoiNjVmN...==",
    "hasMore": true
  }
}
```

---

#### 🛠️ Database Implementation Guides

##### A. PostgreSQL / MySQL / SQLite (with `table.BuildSQLQuery` + `table.ExecuteGeneric`)
`table.BuildSQLQuery` automatically generates parameterized SQL with multi-field search, column filters, tenant scoping, and SQL injection protection:

```go
package controllers

import (
	"context"
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/kamlesh-Sahani/go-commons/table"
)

type Company struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Revenue   float64 `json:"revenue"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

func ListCompanies(c *gin.Context, db *sql.DB) {
	table.ExecuteGeneric(c, table.GenericTableConfig[Company]{
		EntityName:     "companies",
		ExportFileName: "companies_export",
		Columns: []table.TableColumn{
			{ID: "name", Title: "Company Name", Sortable: true},
			{ID: "revenue", Title: "Annual Revenue", Sortable: true},
			{ID: "status", Title: "Status", Sortable: false},
		},
		RowMapper: func(comp Company) map[string][]table.TableCell {
			return map[string][]table.TableCell{
				"name":    {table.NewTextCell(comp.Name, "", true)},
				"revenue": {table.NewCurrencyCell(comp.Revenue, "USD")},
				"status":  {table.NewTagCell(comp.Status, table.BadgeColors["SUCCESS"])},
			}
		},
		Query: func(ctx context.Context, req *table.ListRequest) (*table.QueryResult[Company], error) {
			// 🚀 Safe, parameterized SQL generation for Postgres ($1, $2) or MySQL (?):
			sqlQuery, err := table.BuildSQLQuery(req, table.SQLTableConfig{
				Table:         "companies",
				SelectColumns: []string{"id", "name", "revenue", "status", "created_at"},
				SearchFields:  []string{"name", "status"},
				AllowedSorts:  map[string]string{"name": "name", "revenue": "revenue", "created_at": "created_at"},
				DefaultSort:   "created_at",
				TenantID:      c.GetString("tenant_id"), // Auto-scoped: "tenant_id = $1"
				SoftDelete:    true,                     // Auto-scoped: "deleted_at IS NULL"
				Dialect:       table.DialectPostgres,    // Uses $1, $2 and ILIKE (or table.DialectMySQL)
			})
			if err != nil {
				return nil, err
			}

			// Run Count:
			var total int64
			_ = db.QueryRowContext(ctx, sqlQuery.CountQuery, sqlQuery.Args...).Scan(&total)

			// Run Select:
			rows, err := db.QueryContext(ctx, sqlQuery.SelectQuery, sqlQuery.Args...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()

			var items []Company
			for rows.Next() {
				var item Company
				_ = rows.Scan(&item.ID, &item.Name, &item.Revenue, &item.Status, &item.CreatedAt)
				items = append(items, item)
			}

			return &table.QueryResult[Company]{
				Items:     items,
				TotalRows: total,
				HasMore:   int64(req.Page*req.Limit) < total,
			}, nil
		},
	})
}
```

##### B. MongoDB (with `table.ExecuteTableList`)
For MongoDB collections, `table.ExecuteTableList` coordinates parallel count/find queries, keyset cursors, and aggregation joins automatically:

```go
package controllers

import (
	"time"
	"github.com/gin-gonic/gin"
	"github.com/kamlesh-Sahani/go-commons/table"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Email     string             `bson:"email" json:"email"`
	Status    string             `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

func ListUsers(c *gin.Context) {
	table.ExecuteTableList[User](c, table.TableListConfig[User]{
		CollectionName: "users",
		EntityName:     "users",
		TenantID:       c.GetString("tenant_id"), // Auto-injects {"tenant_id": ...}
		SoftDelete:     true,                     // Auto-injects {"deleted_at": {"$exists": false}}
		SearchFields:   []string{"name", "email"},
		SearchPrefix:   true,                     // Anchors search with ^ for indexed B-Tree scans
		DefaultSort:    "created_at",
		DefaultSortDir: -1,                       // -1 for DESC
		Columns: []table.TableColumn{
			{ID: "name", Title: "Full Name", Sortable: true},
			{ID: "email", Title: "Email Address", Sortable: true},
			{ID: "status", Title: "Status", Sortable: false},
			{ID: "created_at", Title: "Joined Date", Sortable: true},
		},
		RowMapper: func(u User) map[string][]table.TableCell {
			return map[string][]table.TableCell{
				"name":       {table.NewTextCell(u.Name, "", true)},
				"email":      {table.NewCopyableCell(u.Email)},
				"status":     {table.NewStatusCell(u.Status, "#dcfce7", "#15803d", "#86efac")},
				"created_at": {table.NewDateCell(u.CreatedAt, "02 Jan 2006")},
			}
		},
	})
}
```

##### C. In-Memory Slices & Microservice APIs (with `table.ExecuteSlice`)
When records come from in-memory caches, CSVs, or external microservices:

```go
func ListSystemLogs(c *gin.Context) {
	logs := getAuditLogsFromCache() // []AuditLog

	table.ExecuteSlice(c, logs, table.SliceTableConfig[AuditLog]{
		EntityName: "audit_logs",
		Columns: []table.TableColumn{
			{ID: "event", Title: "Event Action"},
			{ID: "user", Title: "Triggered By"},
		},
		RowMapper: func(item AuditLog) map[string][]table.TableCell {
			return map[string][]table.TableCell{
				"event": {table.NewTextCell(item.Action, "")},
				"user":  {table.NewTextCell(item.User, "#6b7280")},
			}
		},
	})
}
```

---

#### ⚡ Performance & Enterprise Features

##### 1. Keyset Cursor Pagination ($O(1)$ vs $O(N)$ OFFSET)
Traditional `OFFSET 100000 LIMIT 20` forces the database to read 100,020 rows and discard the first 100,000, causing severe query degradation on large tables.

The `table` keyset cursor encodes the boundary into a compact base64 token:
`{"i": "last_id", "f": "created_at", "v": "2026-10-04T12:00:00Z"}`

The engine executes:
```sql
WHERE (created_at < $1) OR (created_at = $1 AND id < $2)
ORDER BY created_at DESC, id DESC
LIMIT 21
```
The database jumps directly to the boundary via the B-Tree index in **constant $O(1)$ time** regardless of whether page 1 or page 50,000 is being fetched.

##### 2. Zero-Memory Streaming Export (`table.StreamExport`)
Loading 50,000 rows into memory for CSV/Excel export causes memory spikes and container Out-Of-Memory (OOM) kills.

`table.StreamExport` reads records one-by-one from the database cursor, converts each row to CSV or Excel XML, and flushes it immediately to the HTTP socket via chunked transfer encoding (`Transfer-Encoding: chunked`). **Memory usage remains constant at ~64KB RAM even when exporting 1,000,000 records.**

##### 3. UI Cell Builder Reference (`table/cells.go`)
Construct rich, semantic table cells directly from the backend:
- `table.NewTextCell(text, color, isBold)` — Formatted text cell
- `table.NewLinkCell(text, url, color, isBold)` — Clickable link cell
- `table.NewCurrencyCell(amount, "USD")` — Localized currency formatting
- `table.NewDateCell(timeObj, "02 Jan 2006")` — Formatted date
- `table.NewRelativeDateCell(timeObj)` — Relative humanized time ("5m ago", "2d ago")
- `table.NewAvatarCell(name, imageURL, subtitle)` — User avatar cell
- `table.NewCopyableCell(apiKey, "Click to copy")` — Copy-to-clipboard widget
- `table.NewTagCell(text, table.BadgeColors["SUCCESS"])` — Colored pill badges
- `table.NewStatusCell(title, bgColor, textColor, borderColor)` — Custom badge
- `table.NewProgressCell(85)` — Percentage progress bar
- `table.NewActionCell("delete", "CONFIRM", "Are you sure?", apiData)` — Action trigger buttons

---

### 2. 💬 Standard API Responses (`response`)

Never write repetitive `c.JSON(http.StatusOK, gin.H{"success": true, ...})` again:

```go
import "github.com/kamlesh-Sahani/go-commons/response"

// 200 OK
response.Success(c, "Companies fetched successfully", companies)

// 201 Created
response.Created(c, "Invoice created successfully", newInvoice)

// 400 Bad Request
response.BadRequest(c, "Invalid input: email is required")

// 401 Unauthorized
response.Unauthorized(c, "Authentication token expired")

// 404 Not Found
response.NotFound(c, "Customer not found")

// 500 Internal Server Error
response.InternalServerError(c, "Failed to connect to database")
```

All responses follow a clean, consistent JSON envelope:
```json
{
  "success": true,
  "message": "Companies fetched successfully",
  "data": { ... }
}
```

---

### 3. 🔍 Validation & Utility Helpers (`utils`)

#### Form Validation with `notblank`
Includes pre-registered Gin validation and human-friendly error messages:

```go
import "github.com/kamlesh-Sahani/go-commons/utils"

type SignupRequest struct {
    Name  string `json:"name" binding:"required,notblank"` // Rejects spaces-only like "   "
    Email string `json:"email" binding:"required,email"`
}

func Signup(c *gin.Context) {
    var req SignupRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // Formats validator errors into readable text: "name cannot be empty or whitespace only"
        response.BadRequest(c, utils.FormatValidationError(err))
        return
    }
}
```

#### Currency & Financial Calculations
Store money safely in integer cents to eliminate floating-point rounding errors:

```go
cents := utils.DecimalToCents(49.99)          // 4999 (integer)
dollars := utils.CentsToDecimal(4999)         // 49.99 (float)
display := utils.FormatCurrency(4999, "USD")  // "$49.99"
tax := utils.CalculateTax(10000, 18.0)        // 1800 (18% of $100.00)
```

#### String & Date Helpers
```go
utils.Slugify("Hello World & Go!")   // "hello-world-and-go"
utils.MaskEmail("kamlesh@gmail.com") // "k*****h@gmail.com"
utils.MaskCard("4111111111111234")   // "•••• •••• •••• 1234"
utils.IsValidDate("2026-10-04")      // true (supports ISO-8601 & RFC3339)
utils.TimeAgo(time.Now().Add(-2*time.Hour)) // "2 hours ago"
```

---

### 4. 📦 File Upload Client SDK (`sdk`)

When a user uploads a file (e.g. invoice PDF, product picture, avatar), your backend communicates with the hosted upload service using the client SDK:

```go
import "github.com/kamlesh-Sahani/go-commons/sdk"

// 1. Initialize client once
var uploadClient = sdk.NewClient("https://upload-api.yourcompany.com", "your_secret_api_key")

// 2. Request a 15-minute presigned upload URL for the frontend:
func GetUploadURL(c *gin.Context) {
    res, err := uploadClient.GetPresignedUpload(c.Request.Context(), sdk.UploadRequest{
        FileName:    "invoice_101.pdf",
        ContentType: "application/pdf",
        Folder:      "invoices",
        MaxBytes:    10 * 1024 * 1024, // 10MB limit
    })
    if err != nil {
        response.InternalServerError(c, "Failed to get upload URL")
        return
    }
    response.Success(c, "Upload URL generated", res)
}

// 3. Confirm file upload after the frontend finishes uploading:
func SaveInvoice(c *gin.Context) {
    fileKey := c.PostForm("fileKey") // e.g. "projects/billing/invoices/abc123_invoice_101.pdf"

    // Confirm upload in 1 line (marks file status as confirmed in S3 so it is never deleted):
    res, err := uploadClient.Confirm(c.Request.Context(), fileKey)
    if err != nil {
        response.InternalServerError(c, "Upload confirmation failed")
        return
    }

    response.Created(c, "Invoice saved successfully", res)
}
```

---

### 5. 🛡️ Security Middlewares (`middleware`)

Drop-in middlewares for Gin routers:

```go
import "github.com/kamlesh-Sahani/go-commons/middleware"

router := gin.New()

router.Use(middleware.Recovery())                    // Catches panics and returns clean JSON 500
router.Use(middleware.CORS())                        // Standard CORS configuration
router.Use(middleware.MaxBodySize(10 * 1024 * 1024)) // Reject payloads larger than 10MB
router.Use(middleware.RateLimiter(100, time.Second)) // 100 requests/sec sliding-window limiter
```

---

### 6. 🔐 Password Hashing & Encryption (`crypto`)

```go
import "github.com/kamlesh-Sahani/go-commons/crypto"

// 1. Bcrypt Password Hashing
hashedPassword, _ := crypto.HashPassword("mySuperSecretPassword")
isValid := crypto.CheckPasswordHash("mySuperSecretPassword", hashedPassword) // true

// 2. AES-256-GCM Encryption (for sensitive database fields like tokens or keys)
secretKey := []byte("12345678901234567890123456789012") // 32 bytes
encrypted, _ := crypto.EncryptAESGCM("secret-bank-data", secretKey)
decrypted, _ := crypto.DecryptAESGCM(encrypted, secretKey) // "secret-bank-data"

// 3. Secure Random Token
token, _ := crypto.GenerateRandomHex(32) // 64-character secure random hex
```

---

### 7. 🐇 RabbitMQ Queue (`queue`)

Thread-safe message publishing and resilient background workers with automatic Ack/Nack:

```go
import "github.com/kamlesh-Sahani/go-commons/queue"

// 1. Connect
rmq, err := queue.NewRabbitClient("amqp://guest:guest@localhost:5672/")
defer rmq.Close()

// 2. Publish JSON task
err = rmq.PublishJSON(ctx, "billing_exchange", "invoice.generate", map[string]any{
    "invoiceID": 101,
    "amount":    5000,
})

// 3. Start background worker (10 concurrent workers)
err = rmq.StartConsumer(ctx, "invoice_queue", 10, func(ctx context.Context, body []byte) error {
    log.Printf("Processing invoice message: %s", string(body))
    return nil // return nil to Ack, return error to Nack & Requeue
})
```

---

## 🚀 Running the Hosted Upload API Server

If you want to run the dedicated file upload microservice:

### 1. Configuration (`.env`)
```env
PORT=8080
ENV=development

# Storage / S3 / MinIO Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_S3_BUCKET=your-media-bucket

# Optional: MinIO local development
# AWS_ENDPOINT=http://localhost:9000
# AWS_S3_FORCE_PATH_STYLE=true

# API Key Authentication
AUTH_ENABLED=true
API_KEYS=billing_secret_key:billing-project,crm_secret_key:crm-project
```

### 2. Start the Server
```bash
# Direct run
go run ./cmd/server

# Or with Docker Compose (includes MinIO & RabbitMQ)
docker compose up -d
```

### 3. Server Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/upload/presigned-url` | Generates 15-minute S3 upload URL; tags object as `status=pending` |
| `POST` | `/api/v1/upload/confirm` | Confirms upload; changes tag to `status=confirmed` |
| `POST` | `/api/v1/upload/presigned-download-url` | Generates temporary signed download URL for private files |
| `GET`  | `/api/v1/upload/info?key=...` | Retrieves file size, content-type, and metadata |
| `DELETE`| `/api/v1/upload?key=...` | Deletes file from S3 |
| `POST` | `/api/v1/upload/batch-delete` | Deletes multiple files in one request |

---

## 🔒 Security Architecture

```
github.com/kamlesh-Sahani/go-commons
├── internal/                 🔒 PRIVATE TO SERVER (Cannot be imported outside!)
│   ├── storage/              # Master S3 credentials, presigning logic
│   ├── handlers/             # Server HTTP controllers
│   └── routes/               # API route definitions
│
├── cmd/server/               # 🚀 Standalone Upload Microservice
│
└── public packages           📦 REUSABLE GO MODULE (Importable by your projects)
    ├── table/                # Universal table engine (Postgres, MySQL, Mongo, Slices)
    ├── response/             # Standard JSON responses
    ├── utils/                # Validators, currency, date math
    ├── sdk/                  # Client SDK to call the upload service
    ├── middleware/           # CORS, rate limiter, panic recovery
    ├── crypto/               # Bcrypt & AES-256-GCM
    └── queue/                # RabbitMQ publisher & worker loop
```

Because master S3 credentials and upload handlers are placed inside the **`internal/`** directory, the Go compiler strictly blocks other projects from importing them. Your master AWS credentials can never leak into consumer projects.

---

## 🧪 Testing

Run all unit tests across all packages:

```bash
go test -v ./...
```
