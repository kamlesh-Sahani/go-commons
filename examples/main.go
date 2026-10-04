package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kamlesh-dev/go-common/crypto"
	"github.com/kamlesh-dev/go-common/middleware"
	"github.com/kamlesh-dev/go-common/response"
	"github.com/kamlesh-dev/go-common/sdk"
	"github.com/kamlesh-dev/go-common/table"
)

type Company struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Revenue   float64   `json:"revenue"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func main() {
	router := gin.New()

	// 1. Reusable Middlewares
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RateLimiter(100, time.Second))

	// 2. Initialize Common API SDK (Talks to the hosted common API microservice)
	commonClient := sdk.NewClient("http://localhost:8080", "acc_dev_secret_key")

	// 3. Example File Confirmation Route
	router.POST("/api/invoices", func(c *gin.Context) {
		type InvoiceForm struct {
			ClientName string `json:"clientName" binding:"required"`
			Amount     int    `json:"amount" binding:"required"`
			FileKey    string `json:"fileKey" binding:"required"`
		}

		var form InvoiceForm
		if err := c.ShouldBindJSON(&form); err != nil {
			response.BadRequest(c, "Invalid invoice form: "+err.Error())
			return
		}

		// 1-line confirmation to Common API via SDK!
		confirmRes, err := commonClient.Confirm(c.Request.Context(), form.FileKey)
		if err != nil {
			response.InternalServerError(c, "Failed to confirm file upload: "+err.Error())
			return
		}

		response.Created(c, "Invoice created successfully", gin.H{
			"clientName": form.ClientName,
			"amount":     form.Amount,
			"attachment": confirmRes,
		})
	})

	// 4. Example Database-Agnostic Table (Works with PostgreSQL / pgx / GORM)
	router.POST("/api/companies/table", func(c *gin.Context) {
		table.ExecuteGeneric(c, table.GenericTableConfig[Company]{
			EntityName:     "companies",
			ExportFileName: "companies_export",
			Columns: []table.TableColumn{
				{ID: "name", Title: "Company Name", Sortable: true, Width: 200},
				{ID: "email", Title: "Admin Email", Sortable: true, Width: 200},
				{ID: "revenue", Title: "Annual Revenue", Sortable: true, Width: 150},
				{ID: "status", Title: "Status", Sortable: true, Width: 120},
				{ID: "created_at", Title: "Created At", Sortable: true, Width: 150},
			},
			RowMapper: func(comp Company) map[string][]table.TableCell {
				row := make(map[string][]table.TableCell)
				row["name"] = []table.TableCell{table.NewTextCell(comp.Name, "", true)}
				row["email"] = []table.TableCell{table.NewTextCell(comp.Email, "#6b7280")}
				row["revenue"] = []table.TableCell{table.NewCurrencyCell(comp.Revenue, "USD")}
				if comp.Status == "active" {
					row["status"] = []table.TableCell{table.Badge("Active", table.BadgeColors["SUCCESS"])}
				} else {
					row["status"] = []table.TableCell{table.Badge("Inactive", table.BadgeColors["REJECTED"])}
				}
				row["created_at"] = []table.TableCell{table.NewDateCell(comp.CreatedAt)}
				return row
			},
			Query: func(ctx context.Context, req *table.ListRequest) (*table.QueryResult[Company], error) {
				// (A) Generate safe parameterized SQL for PostgreSQL / pgx / GORM in 1 line:
				sqlQuery, _ := table.BuildSQLQuery(req, table.SQLTableConfig{
					Table:         "companies",
					SearchFields:  []string{"name", "email"},
					AllowedSorts:  map[string]string{"name": "name", "revenue": "revenue", "created_at": "created_at"},
					DefaultSort:   "created_at",
					TenantID:      "tenant-uuid-1",
					SoftDelete:    true,
					Dialect:       table.DialectPostgres, // or table.DialectMySQL
				})

				fmt.Printf("[SQL Generated] Count: %s | Args: %v\n", sqlQuery.CountQuery, sqlQuery.Args)
				fmt.Printf("[SQL Generated] Select: %s\n", sqlQuery.SelectQuery)

				// (B) In real code, execute via database.DB.QueryRow(ctx, sqlQuery.CountQuery, ...)
				// Here we return mock data for demonstration:
				mockData := []Company{
					{ID: "1", Name: "Acme Corp", Email: "contact@acme.com", Revenue: 250000, Status: "active", CreatedAt: time.Now()},
					{ID: "2", Name: "Globex Inc", Email: "info@globex.com", Revenue: 780000, Status: "inactive", CreatedAt: time.Now().Add(-24 * time.Hour)},
				}

				return &table.QueryResult[Company]{
					Items:     mockData,
					TotalRows: 2,
					HasMore:   false,
				}, nil
			},
		})
	})

	// 5. Example In-Memory Slice Table
	mockStaticList := []Company{
		{ID: "1", Name: "Alpha", Email: "alpha@test.com", Revenue: 1000, Status: "active", CreatedAt: time.Now()},
		{ID: "2", Name: "Beta", Email: "beta@test.com", Revenue: 2000, Status: "active", CreatedAt: time.Now()},
	}
	router.POST("/api/static-list", func(c *gin.Context) {
		table.ExecuteSlice(c, mockStaticList, table.SliceTableConfig[Company]{
			EntityName: "static_companies",
			Columns: []table.TableColumn{
				{ID: "name", Title: "Name"},
				{ID: "revenue", Title: "Revenue"},
			},
			RowMapper: func(item Company) map[string][]table.TableCell {
				return map[string][]table.TableCell{
					"name":    {table.NewTextCell(item.Name, "")},
					"revenue": {table.NewCurrencyCell(item.Revenue, "USD")},
				}
			},
		})
	})

	// 6. Example Crypto Usage
	router.POST("/api/hash-test", func(c *gin.Context) {
		hash, _ := crypto.HashPassword("mySecretPassword")
		response.Success(c, "Password hashed", gin.H{"hash": hash})
	})

	fmt.Println("🚀 Example project running on http://localhost:8081")
	_ = router.Run(":8081")
}
