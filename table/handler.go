package table

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kamlesh-Sahani/go-commons/response"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DefaultCollectionResolver is a global hook to resolve *mongo.Collection by name across projects.
var DefaultCollectionResolver func(name string) *mongo.Collection

// SetDefaultCollectionResolver registers a default collection resolver (e.g. database.GetCollection).
func SetDefaultCollectionResolver(fn func(name string) *mongo.Collection) {
	DefaultCollectionResolver = fn
}

// TableListConfig defines configuration passed by a controller to ExecuteTableList.
type TableListConfig[T any] struct {
	Collection      *mongo.Collection                                      // Direct collection instance (optional)
	CollectionName  string                                                 // MongoDB collection name (e.g. "customers")
	GetCollection   func(name string) *mongo.Collection                    // Resolver hook (optional)
	EntityName      string                                                 // Entity display name (e.g. "customers")
	ExportFileName  string                                                 // Export filename without extension
	BaseFilter      bson.M                                                 // Scoped filter (e.g. tenant_id + deleted_at)
	TenantID        any                                                    // If set, auto-injects {"tenant_id": TenantID} into base filter
	SoftDelete      bool                                                   // If true, auto-injects {"deleted_at": bson.M{"$exists": false}}
	SearchFields    []string                                               // Fields to search against
	SearchPrefix    bool                                                   // If true, anchors search with ^ for indexed scan
	AllowedSorts    map[string]string                                      // Map of frontend column IDs to DB fields
	DefaultSort     string                                                 // Default DB sort field (e.g. "created_at")
	DefaultSortDir  int                                                    // 1 for ASC, -1 for DESC (defaults to -1)
	Projection      bson.M                                                 // Optional projection to minimize payload
	Pipeline        []bson.D                                               // Optional aggregation pipeline stages (e.g. $lookup joins)
	Columns         []TableColumn                                          // Column definitions
	RowMapper       func(item T) map[string][]TableCell                    // Function to map entity to row cells
	CursorExtractor func(item T, sortField string) (id string, sortVal any) // Optional fast-path cursor extractor (bypasses reflection)
	TransformQuery  func(filter bson.M) bson.M                             // Optional hook for custom query filters
	ExtraData       map[string]any                                         // Additional metadata returned in TableResponseData
}

// ParseAndValidateListRequest parses and validates listing & export requests from the POST JSON body.
func ParseAndValidateListRequest(c *gin.Context) (*ListRequest, bool) {
	if c.Request.Method != "POST" {
		response.BadRequest(c, "information is not received")
		return nil, false
	}

	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "information is not received")
		return nil, false
	}

	// 1. Export Request Validation
	if req.Export != nil && req.Export.IsExport {
		if strings.TrimSpace(string(req.Export.Format)) == "" {
			response.BadRequest(c, "Export format is required")
			return nil, false
		}
		if req.Limit <= 0 && req.Export.Count <= 0 {
			response.BadRequest(c, "Export count is required")
			return nil, false
		}
		if req.Export.Count > 50000 {
			req.Export.Count = 50000
		}
		return &req, true
	}

	// 2. Listing Request Validation
	if req.Limit <= 0 {
		response.BadRequest(c, "Limit is required")
		return nil, false
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	if req.Page <= 0 && strings.TrimSpace(string(req.Cursor)) == "" {
		response.BadRequest(c, "Information is not received")
		return nil, false
	}

	return &req, true
}

// ExecuteTableList is a generic high-performance table handler delivering:
// 1. Strict POST-only validation
// 2. Keyset / Cursor Pagination + Offset pagination
// 3. Parallel CountDocuments (with fast estimation and 3s timeout) + Find execution
// 4. SkipCount support for pure O(1) cursor pagination infinite scroll
// 5. Zero-allocation streaming export (CSV/Excel) preventing Out-Of-Memory spikes
func ExecuteTableList[T any](c *gin.Context, cfg TableListConfig[T]) {
	req, ok := ParseAndValidateListRequest(c)
	if !ok {
		return
	}

	collection, err := resolveMongoCollection(cfg)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	// 1. Build Query Filter
	baseFilter := buildMongoBaseFilter(cfg)
	filter := BuildListingFilter(
		baseFilter,
		req.Search,
		cfg.SearchFields,
		cfg.SearchPrefix,
		req.Filters,
		cfg.AllowedSorts,
		cfg.TransformQuery,
	)

	// Quick total count check
	if req.FetchTotal {
		handleFetchTotal(c, collection, cfg, filter)
		return
	}

	// 2. Sort and pagination setup
	sortField, sortOrder := resolveSort(cfg, req)
	isCursorMode := req.PaginationType == PaginationModeCursor
	isExporting := req.Export != nil && req.Export.IsExport
	queryLimit := int64(req.Limit)

	findFilter := bson.M{}
	for k, v := range filter {
		findFilter[k] = v
	}
	applyKeysetCursorFilter(findFilter, req, sortField, sortOrder)

	findOptions := buildMongoFindOptions(cfg, req, sortField, sortOrder, isCursorMode, isExporting, queryLimit)

	// 3. Handle Streaming Export
	if isExporting {
		handleMongoExport(c, collection, cfg, req, findFilter, findOptions, sortField, sortOrder)
		return
	}

	// 4. Parallel Query Execution
	shouldCount := !req.SkipCount && !isCursorMode
	totalCount, items, hasMore, countErr, findErr := executeMongoQuery(
		c.Request.Context(),
		collection,
		cfg,
		filter,
		findFilter,
		findOptions,
		req,
		sortField,
		sortOrder,
		queryLimit,
		isCursorMode,
		shouldCount,
	)

	if countErr != nil {
		log.Printf("[ERROR] [ExecuteTableList] Failed to count documents in %s: %v", cfg.CollectionName, countErr)
		response.InternalServerError(c, fmt.Sprintf("Unable to retrieve %s. Please try again later.", cfg.EntityName))
		return
	}
	if findErr != nil {
		log.Printf("[ERROR] [ExecuteTableList] Failed to fetch documents in %s: %v", cfg.CollectionName, findErr)
		response.InternalServerError(c, fmt.Sprintf("Unable to retrieve %s. Please try again later.", cfg.EntityName))
		return
	}

	// 5. Generate cursor and send response
	nextCursor := extractNextCursor(items, sortField, hasMore, cfg.CursorExtractor)
	respondTableList(c, cfg, req, items, totalCount, nextCursor, hasMore)
}

func resolveMongoCollection[T any](cfg TableListConfig[T]) (*mongo.Collection, error) {
	if cfg.Collection != nil {
		return cfg.Collection, nil
	}
	if cfg.GetCollection != nil && cfg.CollectionName != "" {
		if col := cfg.GetCollection(cfg.CollectionName); col != nil {
			return col, nil
		}
	}
	if DefaultCollectionResolver != nil && cfg.CollectionName != "" {
		if col := DefaultCollectionResolver(cfg.CollectionName); col != nil {
			return col, nil
		}
	}
	return nil, fmt.Errorf("Collection %s is not available", cfg.CollectionName)
}

func buildMongoBaseFilter[T any](cfg TableListConfig[T]) bson.M {
	baseFilter := bson.M{}
	if len(cfg.BaseFilter) > 0 {
		for k, v := range cfg.BaseFilter {
			baseFilter[k] = v
		}
	}
	if cfg.TenantID != nil {
		baseFilter["tenant_id"] = cfg.TenantID
	}
	if cfg.SoftDelete {
		baseFilter["deleted_at"] = bson.M{"$exists": false}
	}
	return baseFilter
}

func handleFetchTotal[T any](c *gin.Context, collection *mongo.Collection, cfg TableListConfig[T], filter bson.M) {
	countCtx, countCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer countCancel()

	var totalCount int64
	var err error

	if len(filter) == 0 {
		totalCount, err = collection.EstimatedDocumentCount(countCtx)
	} else {
		totalCount, err = collection.CountDocuments(countCtx, filter)
	}

	if err != nil {
		log.Printf("[ERROR] [ExecuteTableList] FetchTotal count failed in %s: %v", cfg.CollectionName, err)
		response.InternalServerError(c, "Unable to fetch total count.")
		return
	}

	response.Success(c, "Total count fetched", gin.H{
		"totalRows": int(totalCount),
	})
}

func resolveSort[T any](cfg TableListConfig[T], req *ListRequest) (sortField string, sortOrder int) {
	sortField = cfg.DefaultSort
	if sortField == "" {
		sortField = "created_at"
	}
	if dbCol, ok := cfg.AllowedSorts[req.SortBy]; ok {
		sortField = dbCol
	}

	defaultDir := cfg.DefaultSortDir
	if defaultDir == 0 {
		defaultDir = -1
	}
	sortOrder = BuildSortOrder(req.Order, defaultDir == -1)
	return sortField, sortOrder
}

func applyKeysetCursorFilter(findFilter bson.M, req *ListRequest, sortField string, sortOrder int) {
	if req.PaginationType != PaginationModeCursor || strings.TrimSpace(string(req.Cursor)) == "" {
		return
	}
	cursorID, cursorSortField, cursorSortVal, err := DecodeCursor(string(req.Cursor))
	if err == nil && !cursorID.IsZero() {
		if cursorSortField == "" {
			cursorSortField = sortField
		}
		cursorFilter := BuildCursorFilter(cursorID, cursorSortField, cursorSortVal, sortOrder)
		for k, v := range cursorFilter {
			findFilter[k] = v
		}
	}
}

func buildMongoFindOptions[T any](
	cfg TableListConfig[T],
	req *ListRequest,
	sortField string,
	sortOrder int,
	isCursorMode bool,
	isExporting bool,
	queryLimit int64,
) *options.FindOptions {
	findOptions := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortOrder}}).
		SetBatchSize(int32(req.Limit + 1))

	if cfg.Projection != nil {
		findOptions.SetProjection(cfg.Projection)
	}

	if isExporting {
		findOptions.SetSkip(0)
		if req.Export.Count > 0 {
			findOptions.SetLimit(int64(req.Export.Count))
		}
	} else if isCursorMode {
		findOptions.SetSkip(0)
		findOptions.SetLimit(queryLimit + 1)
	} else {
		querySkip := int64((req.Page - 1) * req.Limit)
		findOptions.SetSkip(querySkip)
		findOptions.SetLimit(queryLimit + 1)
	}

	return findOptions
}

func handleMongoExport[T any](
	c *gin.Context,
	collection *mongo.Collection,
	cfg TableListConfig[T],
	req *ListRequest,
	findFilter bson.M,
	findOptions *options.FindOptions,
	sortField string,
	sortOrder int,
) {
	var cursor *mongo.Cursor
	var err error

	if len(cfg.Pipeline) > 0 {
		aggPipeline := make([]bson.D, 0, len(cfg.Pipeline)+4)
		aggPipeline = append(aggPipeline, bson.D{{Key: "$match", Value: findFilter}})
		aggPipeline = append(aggPipeline, cfg.Pipeline...)
		aggPipeline = append(aggPipeline, bson.D{{Key: "$sort", Value: bson.D{
			{Key: sortField, Value: sortOrder},
			{Key: "_id", Value: sortOrder},
		}}})
		if req.Export.Count > 0 {
			aggPipeline = append(aggPipeline, bson.D{{Key: "$limit", Value: int64(req.Export.Count)}})
		}
		cursor, err = collection.Aggregate(c.Request.Context(), aggPipeline)
	} else {
		cursor, err = collection.Find(c.Request.Context(), findFilter, findOptions)
	}

	if err != nil {
		log.Printf("[ERROR] [ExecuteTableList] Export failed in %s: %v", cfg.CollectionName, err)
		response.InternalServerError(c, fmt.Sprintf("Unable to export %s. Please try again later.", cfg.EntityName))
		return
	}

	fileName := req.Export.FileName
	if fileName == "" {
		fileName = cfg.ExportFileName
		if fileName == "" {
			fileName = fmt.Sprintf("%s_export", cfg.CollectionName)
		}
	}

	format := strings.ToLower(strings.TrimSpace(string(req.Export.Format)))
	if format != "excel" && format != "xlsx" {
		format = "csv"
	} else {
		format = "excel"
	}

	StreamExportFromCursor(c, fileName, cursor, cfg.Columns, cfg.RowMapper, format)
}

func executeMongoQuery[T any](
	ctx context.Context,
	collection *mongo.Collection,
	cfg TableListConfig[T],
	filter bson.M,
	findFilter bson.M,
	findOptions *options.FindOptions,
	req *ListRequest,
	sortField string,
	sortOrder int,
	queryLimit int64,
	isCursorMode bool,
	shouldCount bool,
) (totalCount int64, items []T, hasMore bool, countErr error, findErr error) {
	var (
		cursor *mongo.Cursor
		wg     sync.WaitGroup
	)

	if shouldCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if len(filter) == 0 {
				totalCount, countErr = collection.EstimatedDocumentCount(ctx)
				return
			}

			countCtx, countCancel := context.WithTimeout(ctx, 3*time.Second)
			defer countCancel()

			totalCount, countErr = collection.CountDocuments(countCtx, filter)
			if countErr != nil && countCtx.Err() != nil {
				log.Printf("[WARN] [ExecuteTableList] CountDocuments timed out in %s, skipping count", cfg.CollectionName)
				totalCount = -1
				countErr = nil
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(cfg.Pipeline) > 0 {
			aggPipeline := make([]bson.D, 0, len(cfg.Pipeline)+5)
			aggPipeline = append(aggPipeline, bson.D{{Key: "$match", Value: findFilter}})
			aggPipeline = append(aggPipeline, cfg.Pipeline...)
			aggPipeline = append(aggPipeline, bson.D{{Key: "$sort", Value: bson.D{
				{Key: sortField, Value: sortOrder},
				{Key: "_id", Value: sortOrder},
			}}})
			if isCursorMode {
				aggPipeline = append(aggPipeline, bson.D{{Key: "$limit", Value: queryLimit + 1}})
			} else {
				querySkip := int64((req.Page - 1) * req.Limit)
				if querySkip > 0 {
					aggPipeline = append(aggPipeline, bson.D{{Key: "$skip", Value: querySkip}})
				}
				aggPipeline = append(aggPipeline, bson.D{{Key: "$limit", Value: queryLimit + 1}})
			}
			if cfg.Projection != nil {
				aggPipeline = append(aggPipeline, bson.D{{Key: "$project", Value: cfg.Projection}})
			}
			cursor, findErr = collection.Aggregate(ctx, aggPipeline)
		} else {
			cursor, findErr = collection.Find(ctx, findFilter, findOptions)
		}
	}()

	wg.Wait()

	if cursor != nil {
		defer cursor.Close(ctx)
	}

	if countErr != nil || findErr != nil {
		return totalCount, nil, false, countErr, findErr
	}

	items = make([]T, 0, queryLimit+1)
	if err := cursor.All(ctx, &items); err != nil {
		return totalCount, nil, false, nil, err
	}

	if int64(len(items)) > queryLimit {
		hasMore = true
		items = items[:queryLimit]
	}

	if !shouldCount || totalCount < 0 {
		totalCount = int64(len(items))
		if hasMore {
			totalCount = queryLimit + 1
		}
	}

	return totalCount, items, hasMore, nil, nil
}

func extractNextCursor[T any](items []T, sortField string, hasMore bool, extractor func(T, string) (string, any)) string {
	if len(items) == 0 || !hasMore {
		return ""
	}
	lastItem := items[len(items)-1]

	// 1. Fast-path user closure if defined
	if extractor != nil {
		idStr, sortVal := extractor(lastItem, sortField)
		if idStr != "" {
			return EncodeCursorGeneric(idStr, sortField, sortVal)
		}
	}

	// 2. Fast-path CursorProvider or cached reflection
	lastID, lastSortVal := ExtractItemCursor(lastItem, sortField)
	if !lastID.IsZero() {
		return EncodeCursor(lastID, sortField, lastSortVal)
	}
	return ""
}

func respondTableList[T any](
	c *gin.Context,
	cfg TableListConfig[T],
	req *ListRequest,
	items []T,
	totalCount int64,
	nextCursor string,
	hasMore bool,
) {
	tableRows := make([]map[string][]TableCell, 0, len(items))
	if cfg.RowMapper != nil {
		for _, item := range items {
			tableRows = append(tableRows, cfg.RowMapper(item))
		}
	}

	hasData := totalCount > 0
	totalPages := CalculateTotalPages(totalCount, req.Limit)
	features := BuildTableFeatures(true, true, true, true, hasData)

	extraData := map[string]any{
		"nextCursor": nextCursor,
		"hasMore":    hasMore,
	}
	if cfg.ExtraData != nil {
		for k, v := range cfg.ExtraData {
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
		TotalRows:   int(totalCount),
		NextCursor:  nextCursor,
		HasMore:     hasMore,
		ExtraData:   extraData,
	}

	entityTitle := cases.Title(language.English).String(cfg.EntityName)
	message := fmt.Sprintf("%s fetched successfully", entityTitle)
	if totalCount == 0 {
		message = fmt.Sprintf("No %s found", strings.ToLower(cfg.EntityName))
	}

	response.Success(c, message, responseData)
}
