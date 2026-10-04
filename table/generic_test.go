package table

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type MockItem struct {
	ID    string
	Name  string
	Email string
}

func TestExecuteGeneric_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.POST("/api/items", func(ctx *gin.Context) {
		ExecuteGeneric[MockItem](ctx, GenericTableConfig[MockItem]{
			EntityName: "items",
			Columns: []TableColumn{
				{ID: "name", Title: "Name", Sortable: true},
				{ID: "email", Title: "Email", Sortable: true},
			},
			RowMapper: func(item MockItem) map[string][]TableCell {
				return map[string][]TableCell{
					"name":  {NewTextCell(item.Name, "")},
					"email": {NewTextCell(item.Email, "#6b7280")},
				}
			},
			Query: func(qCtx context.Context, req *ListRequest) (*QueryResult[MockItem], error) {
				return &QueryResult[MockItem]{
					Items: []MockItem{
						{ID: "1", Name: "Alpha", Email: "alpha@example.com"},
						{ID: "2", Name: "Beta", Email: "beta@example.com"},
					},
					TotalRows: 2,
					HasMore:   false,
				}, nil
			},
		})
	})

	body := []byte(`{"page": 1, "limit": 10}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got: %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool              `json:"success"`
		Message string            `json:"message"`
		Data    TableResponseData `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if !resp.Success || resp.Data.TotalRows != 2 || len(resp.Data.Rows) != 2 {
		t.Fatalf("unexpected response data: %+v", resp.Data)
	}
}

func TestExecuteSlice_PaginationAndExport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	items := []MockItem{
		{ID: "1", Name: "Item 1", Email: "item1@test.com"},
		{ID: "2", Name: "Item 2", Email: "item2@test.com"},
		{ID: "3", Name: "Item 3", Email: "item3@test.com"},
		{ID: "4", Name: "Item 4", Email: "item4@test.com"},
		{ID: "5", Name: "Item 5", Email: "item5@test.com"},
	}

	// 1. Test Pagination (Page 2, Limit 2)
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.POST("/api/slice", func(ctx *gin.Context) {
		ExecuteSlice(ctx, items, SliceTableConfig[MockItem]{
			EntityName: "items",
			Columns: []TableColumn{
				{ID: "name", Title: "Name"},
			},
			RowMapper: func(item MockItem) map[string][]TableCell {
				return map[string][]TableCell{
					"name": {NewTextCell(item.Name, "")},
				}
			},
		})
	})

	body := []byte(`{"page": 2, "limit": 2}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/slice", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", w.Code)
	}

	var resp struct {
		Success bool              `json:"success"`
		Data    TableResponseData `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Rows) != 2 || resp.Data.TotalRows != 5 || !resp.Data.HasMore {
		t.Fatalf("unexpected slice pagination data: %+v", resp.Data)
	}

	// 2. Test CSV Export
	wExp := httptest.NewRecorder()
	cExp, rExp := gin.CreateTestContext(wExp)

	rExp.POST("/api/slice/export", func(ctx *gin.Context) {
		ExecuteSlice(ctx, items, SliceTableConfig[MockItem]{
			EntityName: "items",
			Columns: []TableColumn{
				{ID: "name", Title: "Name"},
			},
			RowMapper: func(item MockItem) map[string][]TableCell {
				return map[string][]TableCell{
					"name": {NewTextCell(item.Name, "")},
				}
			},
		})
	})

	expBody := []byte(`{"export": {"export": true, "format": "CSV", "count": 100}}`)
	reqExp, _ := http.NewRequest(http.MethodPost, "/api/slice/export", bytes.NewBuffer(expBody))
	reqExp.Header.Set("Content-Type", "application/json")
	cExp.Request = reqExp
	rExp.ServeHTTP(wExp, reqExp)

	if wExp.Code != http.StatusOK {
		t.Fatalf("expected export status 200, got: %d", wExp.Code)
	}
	if !strings.Contains(wExp.Body.String(), "Item 1") || !strings.Contains(wExp.Body.String(), "Item 5") {
		t.Fatalf("expected CSV export body to contain all items, got: %s", wExp.Body.String())
	}
}

func TestCursor_GenericUUIDAndInteger(t *testing.T) {
	// UUID cursor
	uuidStr := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	encUUID := EncodeCursorGeneric(uuidStr, "created_at", "2026-10-04T12:00:00Z")
	decID, decField, _, err := DecodeCursorGeneric(encUUID)
	if err != nil || decID != uuidStr || decField != "created_at" {
		t.Fatalf("failed to decode generic UUID cursor: id=%s, field=%s, err=%v", decID, decField, err)
	}

	// Integer ID cursor
	intID := "49281"
	encInt := EncodeCursorGeneric(intID, "price", 99.50)
	decIntID, decIntField, decVal, err := DecodeCursorGeneric(encInt)
	if err != nil || decIntID != intID || decIntField != "price" {
		t.Fatalf("failed to decode generic int cursor: id=%s, field=%s, err=%v", decIntID, decIntField, err)
	}
	if valFloat, ok := decVal.(float64); !ok || valFloat != 99.50 {
		t.Fatalf("expected 99.50, got: %v", decVal)
	}
}
