package table

import (
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// PaginationParams extracts page, limit, sortBy, sortOrder, and search from Gin query params.
type PaginationParams struct {
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
	SortBy    string `json:"sortBy"`
	SortOrder string `json:"sortOrder"`
	Search    string `json:"search"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
	HasNext    bool  `json:"hasNext"`
	HasPrev    bool  `json:"hasPrev"`
}

type PaginatedResult struct {
	Items      interface{}    `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
}

type CursorParams struct {
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type CursorResult struct {
	Items      interface{} `json:"items"`
	NextCursor string      `json:"nextCursor,omitempty"`
	HasMore    bool        `json:"hasMore"`
}

// GetPaginationParams extracts page, limit, sortBy, sortOrder, and search from Gin query params.
func GetPaginationParams(c *gin.Context) PaginationParams {
	page, err := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(DefaultPage)))
	if err != nil || page < 1 {
		page = DefaultPage
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))
	if err != nil || limit < 1 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}

	sortBy := strings.TrimSpace(c.DefaultQuery("sortBy", "createdAt"))
	sortOrder := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sortOrder", "desc")))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	search := strings.TrimSpace(c.Query("search"))

	return PaginationParams{
		Page:      page,
		Limit:     limit,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Search:    search,
	}
}

// GetCursorParams extracts cursor and limit for infinite scroll feeds.
func GetCursorParams(c *gin.Context) CursorParams {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))
	if err != nil || limit < 1 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}

	cursor := strings.TrimSpace(c.Query("cursor"))

	return CursorParams{
		Cursor: cursor,
		Limit:  limit,
	}
}

// BuildPaginatedResult constructs the standard paginated payload.
func BuildPaginatedResult(items interface{}, totalItems int64, params PaginationParams) PaginatedResult {
	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(params.Limit)))
	}

	hasNext := params.Page < totalPages
	hasPrev := params.Page > 1 && totalPages > 1

	return PaginatedResult{
		Items: items,
		Pagination: PaginationMeta{
			Page:       params.Page,
			Limit:      params.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    hasNext,
			HasPrev:    hasPrev,
		},
	}
}

// BuildCursorResult constructs cursor-based result payload.
func BuildCursorResult(items interface{}, nextCursor string, hasMore bool) CursorResult {
	return CursorResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}
