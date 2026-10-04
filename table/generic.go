package table

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kamlesh-dev/go-common/response"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// QueryResult holds data fetched by any custom database driver, repository, or external service.
type QueryResult[T any] struct {
	Items      []T            // Fetched items for current page/cursor
	TotalRows  int64          // Total matching records across all pages
	HasMore    bool           // If true, indicates more data exists beyond limit
	NextCursor string         // Optional next cursor token for keyset pagination
	ExtraData  map[string]any // Optional extra payload returned to frontend
}

// GenericTableConfig defines the configuration for running a table against ANY database or data source.
// Compatible with PostgreSQL (pgx, database/sql), GORM, MySQL, SQLite, MongoDB, REST APIs, or in-memory lists.
type GenericTableConfig[T any] struct {
	EntityName     string                              // Display name (e.g. "users", "companies")
	ExportFileName string                              // Export filename without extension
	Columns        []TableColumn                       // Column definitions sent to frontend
	RowMapper      func(item T) map[string][]TableCell // Translates entity T into UI TableCells

	// Query executes the database query for the request and returns items and pagination info
	Query func(ctx context.Context, req *ListRequest) (*QueryResult[T], error)

	// StreamExport is an optional hook to stream records with O(1) memory during export.
	// If nil, ExecuteGeneric automatically exports the items returned by Query via StreamExportSlice.
	StreamExport func(c *gin.Context, req *ListRequest, filename string, format string) error

	ExtraData map[string]any // Static metadata to include in response
}

// ExecuteGeneric runs a complete table lifecycle for ANY database:
// 1. Validates request (POST payload with pagination, sorting, search, filters, or export).
// 2. If export: streams CSV/Excel directly to HTTP client with O(1) memory.
// 3. Otherwise: executes cfg.Query, maps rows to UI cells, builds pagination features, and returns JSON.
func ExecuteGeneric[T any](c *gin.Context, cfg GenericTableConfig[T]) {
	req, ok := ParseAndValidateListRequest(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// 1. Handle Export
	if req.Export != nil && req.Export.IsExport {
		fileName := req.Export.FileName
		if fileName == "" {
			fileName = cfg.ExportFileName
			if fileName == "" {
				fileName = fmt.Sprintf("%s_export", cfg.EntityName)
			}
		}

		format := strings.ToLower(strings.TrimSpace(string(req.Export.Format)))
		if format != "excel" && format != "xlsx" {
			format = "csv"
		} else {
			format = "excel"
		}

		// Use custom streaming if provided
		if cfg.StreamExport != nil {
			if err := cfg.StreamExport(c, req, fileName, format); err != nil {
				response.InternalServerError(c, fmt.Sprintf("Export failed: %v", err))
			}
			return
		}

		// Fallback: Query and stream slice
		result, err := cfg.Query(ctx, req)
		if err != nil {
			response.InternalServerError(c, fmt.Sprintf("Unable to retrieve %s for export", cfg.EntityName))
			return
		}

		StreamExportSlice(c, fileName, result.Items, cfg.Columns, cfg.RowMapper, format)
		return
	}

	// 2. Fetch Data
	result, err := cfg.Query(ctx, req)
	if err != nil {
		response.InternalServerError(c, fmt.Sprintf("Unable to retrieve %s: %v", cfg.EntityName, err))
		return
	}
	if result == nil {
		result = &QueryResult[T]{}
	}

	// 3. Map to UI Cells
	tableRows := make([]map[string][]TableCell, 0, len(result.Items))
	if cfg.RowMapper != nil {
		for _, item := range result.Items {
			tableRows = append(tableRows, cfg.RowMapper(item))
		}
	}

	// 4. Build Pagination & Response Payload
	hasData := result.TotalRows > 0 || len(result.Items) > 0
	totalPages := CalculateTotalPages(result.TotalRows, req.Limit)
	features := BuildTableFeatures(true, true, true, true, hasData)

	extraData := map[string]any{
		"nextCursor": result.NextCursor,
		"hasMore":    result.HasMore,
	}
	if cfg.ExtraData != nil {
		for k, v := range cfg.ExtraData {
			extraData[k] = v
		}
	}
	if result.ExtraData != nil {
		for k, v := range result.ExtraData {
			extraData[k] = v
		}
	}

	responseData := TableResponseData{
		Columns:     cfg.Columns,
		Rows:        tableRows,
		Features:    features,
		IsPaginated: true,
		PageNumber:  req.Page,
		TotalPages:  totalPages,
		PageSize:    req.Limit,
		TotalRows:   int(result.TotalRows),
		NextCursor:  result.NextCursor,
		HasMore:     result.HasMore,
		ExtraData:   extraData,
	}

	entityTitle := cases.Title(language.English).String(cfg.EntityName)
	message := fmt.Sprintf("%s fetched successfully", entityTitle)
	if result.TotalRows == 0 && len(result.Items) == 0 {
		message = fmt.Sprintf("No %s found", strings.ToLower(cfg.EntityName))
	}

	response.Success(c, message, responseData)
}

// SliceTableConfig defines configuration for in-memory slice table processing.
type SliceTableConfig[T any] struct {
	EntityName     string
	ExportFileName string
	Columns        []TableColumn
	RowMapper      func(item T) map[string][]TableCell
	ExtraData      map[string]any
}

// ExecuteSlice serves an in-memory slice through the standard Table JSON contract and export pipeline.
func ExecuteSlice[T any](c *gin.Context, allItems []T, cfg SliceTableConfig[T]) {
	ExecuteGeneric(c, GenericTableConfig[T]{
		EntityName:     cfg.EntityName,
		ExportFileName: cfg.ExportFileName,
		Columns:        cfg.Columns,
		RowMapper:      cfg.RowMapper,
		ExtraData:      cfg.ExtraData,
		Query: func(ctx context.Context, req *ListRequest) (*QueryResult[T], error) {
			total := int64(len(allItems))
			limit := req.Limit
			if req.Export != nil && req.Export.IsExport && req.Export.Count > 0 {
				limit = req.Export.Count
			} else if limit <= 0 {
				limit = 10
			}

			page := req.Page
			if page <= 0 || (req.Export != nil && req.Export.IsExport) {
				page = 1
			}

			start := (page - 1) * limit
			if start > len(allItems) {
				start = len(allItems)
			}
			end := start + limit
			if end > len(allItems) {
				end = len(allItems)
			}

			pageItems := allItems[start:end]
			hasMore := end < len(allItems)

			return &QueryResult[T]{
				Items:     pageItems,
				TotalRows: total,
				HasMore:   hasMore,
			}, nil
		},
	})
}
