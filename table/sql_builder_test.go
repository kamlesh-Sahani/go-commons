package table

import (
	"strings"
	"testing"
)

func TestBuildSQLQuery_Postgres(t *testing.T) {
	req := &ListRequest{
		Page:   2,
		Limit:  15,
		Search: "acme",
		SortBy: "name",
		Order:  "asc",
		Filters: map[string]ColumnFilter{
			"status": {
				Operator: "EQ",
				Value:    []string{"ACTIVE"},
			},
			"country": {
				Operator: "IN",
				Value:    []string{"US", "CA"},
			},
		},
	}

	cfg := SQLTableConfig{
		Table:          "companies",
		SelectColumns:  []string{"id", "organization_name", "admin_email", "status"},
		SearchFields:   []string{"organization_name", "admin_email"},
		AllowedSorts:   map[string]string{"name": "organization_name", "status": "is_active"},
		DefaultSort:    "created_at",
		DefaultSortDir: "DESC",
		TenantID:       "tenant-uuid-123",
		SoftDelete:     true,
		Dialect:        DialectPostgres,
	}

	res, err := BuildSQLQuery(req, cfg)
	if err != nil {
		t.Fatalf("unexpected error building SQL: %v", err)
	}

	// 1. Check PostgreSQL placeholders ($1, $2, ...)
	if !strings.Contains(res.WhereClause, "$1") || !strings.Contains(res.WhereClause, "$2") {
		t.Fatalf("expected $1, $2 placeholders in postgres where clause, got: %s", res.WhereClause)
	}

	// 2. Check ILIKE for search
	if !strings.Contains(res.WhereClause, "ILIKE") {
		t.Fatalf("expected ILIKE in postgres where clause, got: %s", res.WhereClause)
	}

	// 3. Check Tenant ID and Soft Delete
	if !strings.Contains(res.WhereClause, "tenant_id = $1") {
		t.Fatalf("expected tenant_id = $1, got: %s", res.WhereClause)
	}
	if !strings.Contains(res.WhereClause, "deleted_at IS NULL") {
		t.Fatalf("expected deleted_at IS NULL, got: %s", res.WhereClause)
	}

	// 4. Check IN operator
	if !strings.Contains(res.WhereClause, "IN (") {
		t.Fatalf("expected IN clause for country filter, got: %s", res.WhereClause)
	}

	// 5. Check Order By
	if res.OrderBy != "ORDER BY organization_name ASC" {
		t.Fatalf("expected 'ORDER BY organization_name ASC', got: %s", res.OrderBy)
	}

	// 6. Check Pagination
	if res.Limit != 15 || res.Offset != 15 {
		t.Fatalf("expected limit 15, offset 15, got: limit %d, offset %d", res.Limit, res.Offset)
	}

	// 7. Check Count and Select queries
	if !strings.HasPrefix(res.CountQuery, "SELECT COUNT(*) FROM companies") {
		t.Fatalf("unexpected count query: %s", res.CountQuery)
	}
	if !strings.HasPrefix(res.SelectQuery, "SELECT id, organization_name, admin_email, status FROM companies") {
		t.Fatalf("unexpected select query: %s", res.SelectQuery)
	}
}

func TestBuildSQLQuery_MySQL(t *testing.T) {
	req := &ListRequest{
		Page:   1,
		Limit:  10,
		Search: "john",
		SortBy: "email",
		Order:  "desc",
	}

	cfg := SQLTableConfig{
		Table:          "users",
		SearchFields:   []string{"email"},
		AllowedSorts:   map[string]string{"email": "email"},
		DefaultSort:    "id",
		DefaultSortDir: "ASC",
		SoftDelete:     true,
		Dialect:        DialectMySQL,
	}

	res, err := BuildSQLQuery(req, cfg)
	if err != nil {
		t.Fatalf("unexpected error building MySQL query: %v", err)
	}

	// MySQL should use '?' placeholders instead of $1
	if strings.Contains(res.WhereClause, "$1") {
		t.Fatalf("mysql where clause should not contain $1: %s", res.WhereClause)
	}
	if !strings.Contains(res.WhereClause, "LIKE ?") {
		t.Fatalf("expected 'LIKE ?' in mysql where clause, got: %s", res.WhereClause)
	}
	if res.OrderBy != "ORDER BY email DESC" {
		t.Fatalf("expected 'ORDER BY email DESC', got: %s", res.OrderBy)
	}
}

func TestBuildSQLQuery_SQLInjectionProtection(t *testing.T) {
	// Attempt SQL injection via filter column name
	req := &ListRequest{
		Page:  1,
		Limit: 10,
		Filters: map[string]ColumnFilter{
			"id = 1; DROP TABLE users; --": {
				Operator: "EQ",
				Value:    []string{"1"},
			},
		},
	}

	cfg := SQLTableConfig{
		Table:       "users",
		DefaultSort: "id",
	}

	_, err := BuildSQLQuery(req, cfg)
	if err == nil {
		t.Fatalf("expected error on malicious filter column key, got nil")
	}

	// Attempt SQL injection via table name
	cfgBadTable := SQLTableConfig{
		Table:       "users; DROP TABLE users; --",
		DefaultSort: "id",
	}
	_, err = BuildSQLQuery(&ListRequest{Limit: 10}, cfgBadTable)
	if err == nil {
		t.Fatalf("expected error on malicious table name, got nil")
	}
}

func TestBuildSQLQuery_AllowedFilters(t *testing.T) {
	req := &ListRequest{
		Limit: 10,
		Filters: map[string]ColumnFilter{
			"role": {
				Operator: "EQ",
				Value:    []string{"admin"},
			},
			"unapproved_field": {
				Operator: "EQ",
				Value:    []string{"secret"},
			},
		},
	}

	cfg := SQLTableConfig{
		Table:          "users",
		AllowedFilters: map[string]string{"role": "user_role"},
		DefaultSort:    "id",
	}

	res, err := BuildSQLQuery(req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res.WhereClause, "user_role = $1") {
		t.Fatalf("expected mapped filter 'user_role = $1', got: %s", res.WhereClause)
	}
	if strings.Contains(res.WhereClause, "unapproved_field") {
		t.Fatalf("unapproved filter should not be present in where clause: %s", res.WhereClause)
	}
}
