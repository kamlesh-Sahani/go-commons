package table

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// 1. Export Configuration

type ExportFormat string

const (
	ExportFormatCSV   ExportFormat = "CSV"
	ExportFormatExcel ExportFormat = "EXCEL"
)

type ExportConfig struct {
	IsExport bool         `json:"export"`
	Format   ExportFormat `json:"format"`
	Count    int          `json:"count"`
	FileName string       `json:"fileName"`
}

// 2. Filter Definitions

type FilterOperator string

const (
	FilterOpEqual          FilterOperator = "EQUAL"
	FilterOpNotEqual       FilterOperator = "NOT_EQUAL"
	FilterOpContains       FilterOperator = "CONTAINS"
	FilterOpStartsWith     FilterOperator = "STARTS_WITH"
	FilterOpIn             FilterOperator = "IN"
	FilterOpNotIn          FilterOperator = "NOT_IN"
	FilterOpGreaterThan    FilterOperator = "GREATER_THAN"
	FilterOpGreaterOrEqual FilterOperator = "GREATER_OR_EQUAL"
	FilterOpLessThan       FilterOperator = "LESS_THAN"
	FilterOpLessOrEqual    FilterOperator = "LESS_OR_EQUAL"
	FilterOpBetween        FilterOperator = "BETWEEN"
)

type ColumnFilter struct {
	Operator FilterOperator `json:"operator"`
	Value    []string       `json:"value"`
}

func (cf *ColumnFilter) UnmarshalJSON(data []byte) error {
	type Alias ColumnFilter
	aux := struct {
		RawValue any `json:"value"`
		*Alias
	}{
		Alias: (*Alias)(cf),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.RawValue == nil {
		cf.Value = nil
		return nil
	}
	switch v := aux.RawValue.(type) {
	case []string:
		cf.Value = v
	case []any:
		var list []string
		for _, item := range v {
			if item != nil {
				list = append(list, fmt.Sprintf("%v", item))
			}
		}
		cf.Value = list
	default:
		str := strings.TrimSpace(fmt.Sprintf("%v", v))
		if str != "" && str != "<nil>" {
			cf.Value = []string{str}
		}
	}
	return nil
}

// 3. Pagination & Request Contract

type FlexibleCursor string

func (fc *FlexibleCursor) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*fc = FlexibleCursor(s)
		return nil
	}
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*fc = ""
		return nil
	}
	return nil
}

type PaginationMode string

const (
	PaginationModeOffset PaginationMode = "offset"
	PaginationModeCursor PaginationMode = "cursor"
)

type ListRequest struct {
	Page           int                     `json:"page"`
	Limit          int                     `json:"limit"`
	Search         string                  `json:"search"`
	SortBy         string                  `json:"sortBy"`
	Order          string                  `json:"order"`
	Cursor         FlexibleCursor          `json:"cursor"`
	Direction      string                  `json:"direction"`
	SkipCount      bool                    `json:"skipCount"`
	FetchTotal     bool                    `json:"fetchTotal"`
	Filters        map[string]ColumnFilter `json:"filters"`
	Export         *ExportConfig           `json:"export"`
	PaginationType PaginationMode          `json:"paginationType"`
}

func CalculateTotalPages(totalCount int64, limit int) int {
	if limit <= 0 || totalCount <= 0 {
		return 0
	}
	return int(math.Ceil(float64(totalCount) / float64(limit)))
}

func BuildSortOrder(order string, defaultDesc ...bool) int {
	isDefaultDesc := true
	if len(defaultDesc) > 0 {
		isDefaultDesc = defaultDesc[0]
	}
	normalized := strings.ToLower(strings.TrimSpace(order))
	if normalized == "asc" || normalized == "ascend" || normalized == "1" {
		return 1
	}
	if normalized == "desc" || normalized == "descend" || normalized == "-1" {
		return -1
	}
	if isDefaultDesc {
		return -1
	}
	return 1
}

// 4. UI Cells and cell builder constructors (NewTextCell, NewLinkCell, etc.) are located in table/cells.go

// 5. Table Columns & Options

type ColumnType string

const (
	ColumnTypeText      ColumnType = "TEXT"
	ColumnTypeDate      ColumnType = "DATE"
	ColumnTypeDateRange ColumnType = "DATE_RANGE"
	ColumnTypeNumber    ColumnType = "NUMBER"
	ColumnTypeSelect    ColumnType = "SELECT"
)

type SelectOption struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TableColumn struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Sortable bool           `json:"sortable,omitempty"`
	Width    int            `json:"width,omitempty"`
	Fixed    string         `json:"fixed,omitempty"`
	Hidden   bool           `json:"hidden,omitempty"`
	Type     ColumnType     `json:"type,omitempty"`
	Options  []SelectOption `json:"options,omitempty"`
}

// 6. Toolbar Features

type ExportFeatures struct {
	CSV   bool `json:"csv"`
	Excel bool `json:"excel"`
	PDF   bool `json:"pdf"`
}

type TableFeatures struct {
	Search       bool           `json:"search"`
	Sort         bool           `json:"sort"`
	Refresh      bool           `json:"refresh"`
	Export       ExportFeatures `json:"export"`
	ColumnToggle bool           `json:"columnToggle"`
	RowSelection bool           `json:"rowSelection"`
}

func BuildTableFeatures(search, sort, refresh, columnToggle, hasData bool) TableFeatures {
	return TableFeatures{
		Search:       search,
		Sort:         sort,
		Refresh:      refresh,
		Export:       ExportFeatures{CSV: hasData, Excel: hasData},
		ColumnToggle: columnToggle,
	}
}

// 7. Table Response Data

type TableResponseData struct {
	Columns     []TableColumn            `json:"columns"`
	Rows        []map[string][]TableCell `json:"rows"`
	Features    TableFeatures            `json:"features"`
	IsPaginated bool                     `json:"isPaginated"`
	PageNumber  int                      `json:"pageNumber"`
	TotalPages  int                      `json:"totalPages"`
	PageSize    int                      `json:"pageSize"`
	TotalRows   int                      `json:"totalRows"`
	NextCursor  string                   `json:"nextCursor,omitempty"`
	PrevCursor  string                   `json:"prevCursor,omitempty"`
	HasMore     bool                     `json:"hasMore"`
	ExtraData   map[string]any           `json:"extraData,omitempty"`
}

// Simple legacy pagination models (PaginationParams, PaginatedResult, CursorParams, CursorResult)
// are located in table/pagination.go.
