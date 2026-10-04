package table

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBuildListingFilter_NoCollision(t *testing.T) {
	baseFilter := bson.M{
		"$or": []bson.M{
			{"tenant_id": "tenant-1"},
			{"is_public": true},
		},
	}

	searchFields := []string{"name", "email"}
	search := "john"

	filters := map[string]ColumnFilter{
		"status": {
			Operator: FilterOpEqual,
			Value:    []string{"ACTIVE"},
		},
		"user_id": {
			Operator: FilterOpIn,
			Value:    []string{"507f1f77bcf86cd799439011"},
		},
	}

	finalFilter := BuildListingFilter(
		baseFilter,
		search,
		searchFields,
		true,
		filters,
		nil,
		nil,
	)

	// Since there are multiple clauses (base filter, search, column filters),
	// it must be safely nested inside an $and clause so $or is NOT overwritten.
	andClauses, ok := finalFilter["$and"].([]bson.M)
	if !ok || len(andClauses) < 3 {
		t.Fatalf("expected $and clause containing at least 3 conditions, got: %v", finalFilter)
	}

	// Verify BaseFilter preserved
	baseOr, hasBaseOr := andClauses[0]["$or"]
	if !hasBaseOr {
		t.Fatalf("expected base filter $or to be preserved, got: %v", andClauses[0])
	}
	if len(baseOr.([]bson.M)) != 2 {
		t.Fatalf("expected 2 base filter conditions, got: %v", baseOr)
	}
}

func TestBuildSearchFilter_MinLength(t *testing.T) {
	// 1-character query should be skipped to prevent full table COLLSCAN
	filter := buildSearchFilter("a", []string{"name"}, false)
	if filter != nil {
		t.Fatalf("expected nil filter for 1-character search, got: %v", filter)
	}

	// 2-character query should produce $or
	filter2 := buildSearchFilter("ab", []string{"name"}, true)
	if filter2 == nil || filter2["$or"] == nil {
		t.Fatalf("expected valid regex $or filter for 2-character search, got: %v", filter2)
	}
}

func TestCursor_DatePreservation(t *testing.T) {
	oid := primitive.NewObjectID()
	now := time.Now().UTC().Truncate(time.Millisecond)

	encoded := EncodeCursor(oid, "created_at", now)
	if encoded == "" {
		t.Fatalf("expected non-empty cursor")
	}

	decodedID, sortField, sortVal, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error decoding cursor: %v", err)
	}

	if decodedID != oid {
		t.Fatalf("expected ID %v, got %v", oid, decodedID)
	}
	if sortField != "created_at" {
		t.Fatalf("expected sortField 'created_at', got %s", sortField)
	}

	decodedTime, isTime := sortVal.(time.Time)
	if !isTime {
		t.Fatalf("expected sortVal to be decoded as time.Time, got %T (%v)", sortVal, sortVal)
	}

	if !decodedTime.Equal(now) {
		t.Fatalf("expected decoded time %v, got %v", now, decodedTime)
	}
}

func TestBuildCursorFilter_ForwardOnly(t *testing.T) {
	oid := primitive.NewObjectID()
	filterDesc := BuildCursorFilter(oid, "_id", nil, -1)
	ltVal, ok := filterDesc["_id"].(bson.M)["$lt"]
	if !ok || ltVal != oid {
		t.Fatalf("expected _id < oid for descending cursor, got: %v", filterDesc)
	}

	filterAsc := BuildCursorFilter(oid, "_id", nil, 1)
	gtVal, ok := filterAsc["_id"].(bson.M)["$gt"]
	if !ok || gtVal != oid {
		t.Fatalf("expected _id > oid for ascending cursor, got: %v", filterAsc)
	}
}

func TestBuildColumnFilters_UppercaseOperatorAndValues(t *testing.T) {
	filters := map[string]ColumnFilter{
		"status": {
			Operator: "EQ",
			Value:    []string{"ACTIVE"},
		},
		"role": {
			Operator: "IN",
			Value:    []string{"ADMIN", "MANAGER"},
		},
		"amount": {
			Operator: "GTE",
			Value:    []string{"100"},
		},
	}

	clauses := buildColumnFilters(filters, nil)
	if len(clauses) != 3 {
		t.Fatalf("expected 3 filter clauses, got %d", len(clauses))
	}

	// Verify status clause
	foundStatus := false
	for _, c := range clauses {
		if val, ok := c["status"]; ok {
			foundStatus = true
			if val != "ACTIVE" {
				t.Fatalf("expected status 'ACTIVE', got %v", val)
			}
		}
	}
	if !foundStatus {
		t.Fatalf("status filter clause was not created")
	}
}

func TestBuildColumnFilters_FrontendCompatibility(t *testing.T) {
	// FE currently sends lowercase strings: "equal", "greater_than", "before", "after", "contains", "start_with", "between"
	filters := map[string]ColumnFilter{
		"status": {
			Operator: "equal",
			Value:    []string{"ACTIVE"},
		},
		"created_at": {
			Operator: "after",
			Value:    []string{"2026-01-01"},
		},
		"name": {
			Operator: "start_with",
			Value:    []string{"Acme"},
		},
	}

	clauses := buildColumnFilters(filters, nil)
	if len(clauses) != 3 {
		t.Fatalf("expected 3 filter clauses, got %d", len(clauses))
	}
}

func TestExportConfig_Constants(t *testing.T) {
	cfg := ExportConfig{
		IsExport: true,
		Format:   ExportFormatCSV,
		Count:    100,
		FileName: "test_export",
	}

	if cfg.Format != ExportFormatCSV {
		t.Fatalf("expected format %s, got %s", ExportFormatCSV, cfg.Format)
	}

	cfgExcel := ExportConfig{
		IsExport: true,
		Format:   ExportFormatExcel,
	}

	if cfgExcel.Format != ExportFormatExcel {
		t.Fatalf("expected format %s, got %s", ExportFormatExcel, cfgExcel.Format)
	}
}

func TestColumnFilter_ArrayAndScalarUnmarshal(t *testing.T) {
	// 1. Unmarshal JSON with array of values
	jsonArray := []byte(`{"operator": "EQUAL", "value": ["ACTIVE", "PENDING"]}`)
	var filterArr ColumnFilter
	if err := json.Unmarshal(jsonArray, &filterArr); err != nil {
		t.Fatalf("failed to unmarshal array value: %v", err)
	}
	if len(filterArr.Value) != 2 || filterArr.Value[0] != "ACTIVE" || filterArr.Value[1] != "PENDING" {
		t.Fatalf("expected 2 array values, got %v", filterArr.Value)
	}

	// 2. Unmarshal JSON with single scalar string value
	jsonScalar := []byte(`{"operator": "EQUAL", "value": "ACTIVE"}`)
	var filterScalar ColumnFilter
	if err := json.Unmarshal(jsonScalar, &filterScalar); err != nil {
		t.Fatalf("failed to unmarshal scalar value: %v", err)
	}
	if len(filterScalar.Value) != 1 || filterScalar.Value[0] != "ACTIVE" {
		t.Fatalf("expected 1 wrapped scalar value, got %v", filterScalar.Value)
	}

	// 3. Verify EQUAL with multiple values converts to $in query
	filters := map[string]ColumnFilter{"status": filterArr}
	clauses := buildColumnFilters(filters, nil)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(clauses))
	}
	inClause, ok := clauses[0]["status"].(bson.M)["$in"]
	if !ok {
		t.Fatalf("expected $in clause for multiple EQUAL values, got %v", clauses[0])
	}
	if len(inClause.([]any)) != 2 {
		t.Fatalf("expected 2 items in $in, got %v", inClause)
	}
}

func TestStatusBadgeColors(t *testing.T) {
	// 1. Verify map elements
	if BadgeColors["SUCCESS"].Bg != "#dcfce7" {
		t.Fatalf("expected SUCCESS bg to be #dcfce7")
	}

	// 2. Verify explicit Badge constructor
	customBadge := Badge("Active", BadgeColors["SUCCESS"])
	if customBadge.BgColor != BadgeColors["SUCCESS"].Bg || customBadge.Title != "Active" {
		t.Fatalf("expected custom badge to use SUCCESS color, got: %v", customBadge)
	}
}

func TestCursor_ArbitraryFieldSort(t *testing.T) {
	oid := primitive.NewObjectID()
	encoded := EncodeCursor(oid, "amount", 1500)

	decodedID, sortField, sortVal, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decodedID != oid || sortField != "amount" {
		t.Fatalf("mismatch in decoded cursor")
	}
	if sortVal != float64(1500) && sortVal != 1500 {
		t.Fatalf("expected 1500, got %v", sortVal)
	}
}

func TestCoerceValue_DatesAndObjectIDs(t *testing.T) {
	hexID := "507f1f77bcf86cd799439011"
	coercedID := coerceValue("user_id", hexID)
	if oid, ok := coercedID.(primitive.ObjectID); !ok || oid.Hex() != hexID {
		t.Fatalf("failed to coerce hex string to ObjectID: %v", coercedID)
	}

	dateStr := "2026-10-04T12:00:00Z"
	coercedDate := coerceValue("created_at", dateStr)
	if dt, ok := coercedDate.(time.Time); !ok || dt.Year() != 2026 {
		t.Fatalf("failed to coerce RFC3339 date string to time.Time: %v", coercedDate)
	}
}

func TestFlexibleCursor_UnmarshalJSON(t *testing.T) {
	strCursorJSON := []byte(`"abc123cursor"`)
	var fc FlexibleCursor
	if err := json.Unmarshal(strCursorJSON, &fc); err != nil || string(fc) != "abc123cursor" {
		t.Fatalf("expected string cursor to parse: %v", fc)
	}

	boolCursorJSON := []byte(`false`)
	var fcBool FlexibleCursor
	if err := json.Unmarshal(boolCursorJSON, &fcBool); err != nil || string(fcBool) != "" {
		t.Fatalf("expected bool false cursor to become empty string: %v", fcBool)
	}
}

func TestEnterpriseCellBuilders(t *testing.T) {
	// Currency cell
	currCell := NewCurrencyCell(149.99, "USD", true)
	if currCell.Type != "CURRENCY" || currCell.Title != "USD 149.99" || !currCell.IsBold {
		t.Fatalf("unexpected currency cell: %+v", currCell)
	}

	// Date cell
	refDate := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	dateCell := NewDateCell(refDate, "02 Jan 2006")
	if dateCell.Type != "DATE" || dateCell.Title != "04 Oct 2026" {
		t.Fatalf("unexpected date cell: %+v", dateCell)
	}

	// Copyable cell
	copyCell := NewCopyableCell("api_secret_key_12345", "Click to copy key")
	if copyCell.Type != "COPYABLE" || copyCell.Title != "Click to copy key" {
		t.Fatalf("unexpected copyable cell: %+v", copyCell)
	}

	// Boolean cell
	boolTrueCell := NewBooleanCell(true, "Enabled", "Disabled")
	if boolTrueCell.Title != "Enabled" || boolTrueCell.BgColor != BadgeColors["SUCCESS"].Bg {
		t.Fatalf("unexpected true boolean cell: %+v", boolTrueCell)
	}

	// Progress cell
	progCell := NewProgressCell(85, "active")
	if progCell.Type != "PROGRESS" || progCell.Title != "85%" {
		t.Fatalf("unexpected progress cell: %+v", progCell)
	}
}

func TestPaginationHelpers(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.GET("/users", func(ctx *gin.Context) {
		p := GetPaginationParams(ctx)
		if p.Page != 2 || p.Limit != 20 || p.SortBy != "created_at" || p.SortOrder != "desc" {
			t.Fatalf("unexpected pagination params: %+v", p)
		}

		result := BuildPaginatedResult([]string{"user1", "user2"}, 50, p)
		if result.Pagination.TotalPages != 3 || !result.Pagination.HasNext || !result.Pagination.HasPrev {
			t.Fatalf("unexpected pagination result: %+v", result.Pagination)
		}
	})

	req, _ := http.NewRequest("GET", "/users?page=2&limit=20&sortBy=created_at&sortOrder=desc", nil)
	c.Request = req
	r.ServeHTTP(w, req)
}
