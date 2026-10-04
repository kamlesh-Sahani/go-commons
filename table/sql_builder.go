package table

import (
	"fmt"
	"regexp"
	"strings"
)

var validSQLIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

// SQLDialect represents the SQL dialect used for placeholder generation and pattern matching.
type SQLDialect string

const (
	DialectPostgres SQLDialect = "postgres" // $1, $2, ... with ILIKE
	DialectMySQL    SQLDialect = "mysql"    // ?, ?, ... with LIKE
	DialectSQLite   SQLDialect = "sqlite"   // ?, ?, ... with LIKE
)

// SQLTableConfig defines configuration for generating parameterized SQL queries from a ListRequest.
type SQLTableConfig struct {
	Table          string            // Database table name (e.g. "companies")
	SelectColumns  []string          // Columns to SELECT (e.g. []string{"id", "name", "email"}). Defaults to ["*"].
	SearchFields   []string          // Columns to search against with ILIKE / LIKE
	AllowedFilters map[string]string // Explicit allowlist of frontend filter IDs to SQL column names
	AllowedSorts   map[string]string // Map from frontend column IDs to SQL column names
	DefaultSort    string            // Default column to sort by (e.g. "created_at")
	DefaultSortDir string            // Default sort direction: "ASC" or "DESC" (defaults to "DESC")
	TenantID       any               // Auto-injected tenant ID: "tenant_id = $X"
	TenantColumn   string            // Defaults to "tenant_id"
	SoftDelete     bool              // If true, automatically appends "deleted_at IS NULL"
	SoftDeleteCol  string            // Defaults to "deleted_at"
	Dialect        SQLDialect        // DialectPostgres (default), DialectMySQL, or DialectSQLite
	BaseWhere      string            // Optional raw WHERE clause prefix (e.g. "status != 'DELETED'")
	BaseArgs       []any             // Optional args corresponding to BaseWhere
}

// SQLQueryResult holds the generated parameterized SQL query and arguments.
type SQLQueryResult struct {
	WhereClause string // Complete WHERE clause with "WHERE ...", or empty string
	Args        []any  // Bound parameter arguments
	OrderBy     string // Complete "ORDER BY ... ASC/DESC"
	Limit       int    // Query limit
	Offset      int    // Query offset
	CountQuery  string // "SELECT COUNT(*) FROM <table> WHERE ..."
	SelectQuery string // "SELECT <columns> FROM <table> WHERE ... ORDER BY ... LIMIT ... OFFSET ..."
}

// BuildSQLQuery translates a database-agnostic ListRequest into safe, parameterized SQL queries
// for PostgreSQL, MySQL, or SQLite. Prevents SQL injection and handles multi-field search and filters.
func BuildSQLQuery(req *ListRequest, cfg SQLTableConfig) (*SQLQueryResult, error) {
	if cfg.Table == "" {
		return nil, fmt.Errorf("table name is required in SQLTableConfig")
	}
	if !validSQLIdentifier.MatchString(cfg.Table) {
		return nil, fmt.Errorf("invalid table name: %q", cfg.Table)
	}

	dialect := cfg.Dialect
	if dialect == "" {
		dialect = DialectPostgres
	}

	tenantCol := cfg.TenantColumn
	if tenantCol == "" {
		tenantCol = "tenant_id"
	}
	if !validSQLIdentifier.MatchString(tenantCol) {
		return nil, fmt.Errorf("invalid tenant column: %q", tenantCol)
	}

	softDelCol := cfg.SoftDeleteCol
	if softDelCol == "" {
		softDelCol = "deleted_at"
	}
	if !validSQLIdentifier.MatchString(softDelCol) {
		return nil, fmt.Errorf("invalid soft delete column: %q", softDelCol)
	}

	selectCols := "*"
	if len(cfg.SelectColumns) > 0 {
		for _, col := range cfg.SelectColumns {
			if !validSQLIdentifier.MatchString(col) && col != "*" {
				return nil, fmt.Errorf("invalid select column: %q", col)
			}
		}
		selectCols = strings.Join(cfg.SelectColumns, ", ")
	}

	var whereClauses []string
	var args []any
	argIndex := 1

	// Helper to generate placeholder based on dialect
	placeholder := func() string {
		if dialect == DialectPostgres {
			ph := fmt.Sprintf("$%d", argIndex)
			argIndex++
			return ph
		}
		argIndex++
		return "?"
	}

	likeOp := "ILIKE"
	if dialect == DialectMySQL || dialect == DialectSQLite {
		likeOp = "LIKE"
	}

	// 1. Base Where & Args
	if strings.TrimSpace(cfg.BaseWhere) != "" {
		whereClauses = append(whereClauses, "("+cfg.BaseWhere+")")
		args = append(args, cfg.BaseArgs...)
		argIndex += len(cfg.BaseArgs)
	}

	// 2. Tenant Scoping
	if cfg.TenantID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", tenantCol, placeholder()))
		args = append(args, cfg.TenantID)
	}

	// 3. Soft Delete
	if cfg.SoftDelete {
		whereClauses = append(whereClauses, fmt.Sprintf("%s IS NULL", softDelCol))
	}

	// 4. Global Search (multi-field OR)
	search := strings.TrimSpace(req.Search)
	searchRunes := []rune(search)
	if len(searchRunes) >= 2 && len(cfg.SearchFields) > 0 {
		if len(searchRunes) > 100 {
			search = string(searchRunes[:100])
		}
		var searchConditions []string
		searchTerm := "%" + search + "%"
		for _, field := range cfg.SearchFields {
			field = strings.TrimSpace(field)
			if field == "" || !validSQLIdentifier.MatchString(field) {
				continue
			}
			searchConditions = append(searchConditions, fmt.Sprintf("%s %s %s", field, likeOp, placeholder()))
			args = append(args, searchTerm)
		}
		if len(searchConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(searchConditions, " OR ")+")")
		}
	}

	// 5. Column-Level Filters
	if len(req.Filters) > 0 {
		for colID, filter := range req.Filters {
			var dbCol string
			if cfg.AllowedFilters != nil {
				mapped, ok := cfg.AllowedFilters[colID]
				if !ok {
					continue
				}
				dbCol = mapped
			} else if mapped, ok := cfg.AllowedSorts[colID]; ok {
				dbCol = mapped
			} else if validSQLIdentifier.MatchString(colID) {
				dbCol = colID
			} else {
				return nil, fmt.Errorf("invalid or disallowed filter column: %q", colID)
			}

			if !validSQLIdentifier.MatchString(dbCol) {
				return nil, fmt.Errorf("invalid filter database column: %q", dbCol)
			}

			// Clean values
			var cleanVals []string
			for _, v := range filter.Value {
				v = strings.TrimSpace(v)
				if v != "" {
					cleanVals = append(cleanVals, v)
				}
			}
			if len(cleanVals) == 0 {
				continue
			}

			op := strings.ToUpper(strings.TrimSpace(string(filter.Operator)))
			firstVal := cleanVals[0]

			switch op {
			case "EQ", "EQUAL":
				if len(cleanVals) == 1 {
					whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", dbCol, placeholder()))
					args = append(args, firstVal)
				} else {
					var inPlaceholders []string
					for _, cv := range cleanVals {
						inPlaceholders = append(inPlaceholders, placeholder())
						args = append(args, cv)
					}
					whereClauses = append(whereClauses, fmt.Sprintf("%s IN (%s)", dbCol, strings.Join(inPlaceholders, ", ")))
				}

			case "NE", "NOT_EQUAL":
				whereClauses = append(whereClauses, fmt.Sprintf("%s != %s", dbCol, placeholder()))
				args = append(args, firstVal)

			case "GT", "GREATER_THAN", "AFTER":
				whereClauses = append(whereClauses, fmt.Sprintf("%s > %s", dbCol, placeholder()))
				args = append(args, firstVal)

			case "GTE", "GREATER_THAN_OR_EQUAL":
				whereClauses = append(whereClauses, fmt.Sprintf("%s >= %s", dbCol, placeholder()))
				args = append(args, firstVal)

			case "LT", "LESS_THAN", "BEFORE":
				whereClauses = append(whereClauses, fmt.Sprintf("%s < %s", dbCol, placeholder()))
				args = append(args, firstVal)

			case "LTE", "LESS_THAN_OR_EQUAL":
				whereClauses = append(whereClauses, fmt.Sprintf("%s <= %s", dbCol, placeholder()))
				args = append(args, firstVal)

			case "CONTAINS":
				whereClauses = append(whereClauses, fmt.Sprintf("%s %s %s", dbCol, likeOp, placeholder()))
				args = append(args, "%"+firstVal+"%")

			case "START_WITH":
				whereClauses = append(whereClauses, fmt.Sprintf("%s %s %s", dbCol, likeOp, placeholder()))
				args = append(args, firstVal+"%")

			case "END_WITH":
				whereClauses = append(whereClauses, fmt.Sprintf("%s %s %s", dbCol, likeOp, placeholder()))
				args = append(args, "%"+firstVal)

			case "BETWEEN":
				if len(cleanVals) >= 2 {
					ph1 := placeholder()
					ph2 := placeholder()
					whereClauses = append(whereClauses, fmt.Sprintf("%s BETWEEN %s AND %s", dbCol, ph1, ph2))
					args = append(args, cleanVals[0], cleanVals[1])
				}

			case "IN":
				var inPlaceholders []string
				for _, cv := range cleanVals {
					inPlaceholders = append(inPlaceholders, placeholder())
					args = append(args, cv)
				}
				whereClauses = append(whereClauses, fmt.Sprintf("%s IN (%s)", dbCol, strings.Join(inPlaceholders, ", ")))

			case "NIN", "NOT_IN":
				var inPlaceholders []string
				for _, cv := range cleanVals {
					inPlaceholders = append(inPlaceholders, placeholder())
					args = append(args, cv)
				}
				whereClauses = append(whereClauses, fmt.Sprintf("%s NOT IN (%s)", dbCol, strings.Join(inPlaceholders, ", ")))
			}
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 6. Sorting
	sortCol := cfg.DefaultSort
	if sortCol == "" {
		sortCol = "created_at"
	}
	if mapped, ok := cfg.AllowedSorts[req.SortBy]; ok {
		sortCol = mapped
	}
	if !validSQLIdentifier.MatchString(sortCol) {
		return nil, fmt.Errorf("invalid sort column: %q", sortCol)
	}

	sortDir := strings.ToUpper(strings.TrimSpace(cfg.DefaultSortDir))
	if sortDir != "ASC" && sortDir != "DESC" {
		sortDir = "DESC"
	}
	reqOrder := strings.ToLower(strings.TrimSpace(req.Order))
	if reqOrder == "asc" || reqOrder == "ascend" || reqOrder == "1" {
		sortDir = "ASC"
	} else if reqOrder == "desc" || reqOrder == "descend" || reqOrder == "-1" {
		sortDir = "DESC"
	}

	orderBy := fmt.Sprintf("ORDER BY %s %s", sortCol, sortDir)

	// 7. Pagination
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if req.Export != nil && req.Export.IsExport && req.Export.Count > 0 {
		limit = req.Export.Count
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	if req.Export != nil && req.Export.IsExport {
		offset = 0
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.Table)
	if whereSQL != "" {
		countQuery += " " + whereSQL
	}

	selectQuery := fmt.Sprintf("SELECT %s FROM %s", selectCols, cfg.Table)
	if whereSQL != "" {
		selectQuery += " " + whereSQL
	}
	selectQuery += fmt.Sprintf(" %s LIMIT %d OFFSET %d", orderBy, limit, offset)

	return &SQLQueryResult{
		WhereClause: whereSQL,
		Args:        args,
		OrderBy:     orderBy,
		Limit:       limit,
		Offset:      offset,
		CountQuery:  countQuery,
		SelectQuery: selectQuery,
	}, nil
}
