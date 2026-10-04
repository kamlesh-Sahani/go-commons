package table

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BuildListingFilter combines base filters, search, and column-level filters into a single MongoDB filter.
func BuildListingFilter(
	baseFilter bson.M,
	search string,
	searchFields []string,
	searchPrefix bool,
	filters map[string]ColumnFilter,
	allowedSorts map[string]string,
	transformFn func(bson.M) bson.M,
) bson.M {
	var andClauses []bson.M

	// 1. Scoped Base Filter (e.g. tenant_id + deleted_at)
	if len(baseFilter) > 0 {
		andClauses = append(andClauses, baseFilter)
	}

	// 2. Search Filter
	if searchClause := buildSearchFilter(search, searchFields, searchPrefix); searchClause != nil {
		andClauses = append(andClauses, searchClause)
	}

	// 3. Column-level Filters
	if colClauses := buildColumnFilters(filters, allowedSorts); len(colClauses) > 0 {
		andClauses = append(andClauses, colClauses...)
	}

	var finalFilter bson.M
	if len(andClauses) == 0 {
		finalFilter = bson.M{}
	} else if len(andClauses) == 1 {
		finalFilter = andClauses[0]
	} else {
		finalFilter = bson.M{"$and": andClauses}
	}

	if transformFn != nil {
		finalFilter = transformFn(finalFilter)
	}

	return finalFilter
}

func buildSearchFilter(search string, searchFields []string, searchPrefix bool) bson.M {
	search = strings.TrimSpace(search)
	searchRunes := []rune(search)
	if len(searchRunes) < 2 || len(searchFields) == 0 {
		return nil
	}

	if len(searchRunes) > 100 {
		search = string(searchRunes[:100])
	}

	escapedSearch := regexp.QuoteMeta(search)
	pattern := escapedSearch
	if searchPrefix {
		pattern = "^" + escapedSearch
	}

	searchRegex := primitive.Regex{Pattern: pattern, Options: "i"}
	orConditions := make([]bson.M, 0, len(searchFields))
	for _, field := range searchFields {
		orConditions = append(orConditions, bson.M{field: searchRegex})
	}

	return bson.M{"$or": orConditions}
}

func buildColumnFilters(filters map[string]ColumnFilter, allowedSorts map[string]string) []bson.M {
	if len(filters) == 0 {
		return nil
	}

	clauses := make([]bson.M, 0, len(filters))
	for colKey, f := range filters {
		if len(f.Value) == 0 {
			continue
		}

		dbCol := colKey
		if mapped, ok := allowedSorts[colKey]; ok {
			dbCol = mapped
		}

		// Prevent NoSQL operator injection (e.g. $where, $expr, $regex)
		if strings.HasPrefix(dbCol, "$") || strings.Contains(dbCol, "\x00") {
			continue
		}

		rawOp := strings.ToUpper(strings.TrimSpace(string(f.Operator)))
		var op FilterOperator
		switch rawOp {
		case "EQUAL", "EQ", "EQUALS", "":
			op = FilterOpEqual
		case "NOT_EQUAL", "NE", "NOT_EQUALS":
			op = FilterOpNotEqual
		case "CONTAINS":
			op = FilterOpContains
		case "STARTS_WITH", "START_WITH":
			op = FilterOpStartsWith
		case "IN":
			op = FilterOpIn
		case "NOT_IN", "NIN":
			op = FilterOpNotIn
		case "GREATER_THAN", "GT", "AFTER":
			op = FilterOpGreaterThan
		case "GREATER_OR_EQUAL", "GTE":
			op = FilterOpGreaterOrEqual
		case "LESS_THAN", "LT", "BEFORE":
			op = FilterOpLessThan
		case "LESS_OR_EQUAL", "LTE":
			op = FilterOpLessOrEqual
		case "BETWEEN", "RANGE":
			op = FilterOpBetween
		default:
			op = FilterOperator(rawOp)
		}

		firstVal := f.Value[0]

		switch op {
		case FilterOpEqual:
			if len(f.Value) > 1 {
				clauses = append(clauses, bson.M{dbCol: bson.M{"$in": coerceSlice(dbCol, f.Value)}})
			} else {
				clauses = append(clauses, bson.M{dbCol: coerceValue(dbCol, firstVal)})
			}

		case FilterOpNotEqual:
			if len(f.Value) > 1 {
				clauses = append(clauses, bson.M{dbCol: bson.M{"$nin": coerceSlice(dbCol, f.Value)}})
			} else {
				clauses = append(clauses, bson.M{dbCol: bson.M{"$ne": coerceValue(dbCol, firstVal)}})
			}

		case FilterOpContains:
			escapedVal := regexp.QuoteMeta(fmt.Sprintf("%v", firstVal))
			clauses = append(clauses, bson.M{dbCol: primitive.Regex{Pattern: escapedVal, Options: "i"}})

		case FilterOpStartsWith:
			escapedVal := regexp.QuoteMeta(fmt.Sprintf("%v", firstVal))
			clauses = append(clauses, bson.M{dbCol: primitive.Regex{Pattern: "^" + escapedVal, Options: "i"}})

		case FilterOpIn:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$in": coerceSlice(dbCol, f.Value)}})

		case FilterOpNotIn:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$nin": coerceSlice(dbCol, f.Value)}})

		case FilterOpGreaterThan:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$gt": coerceValue(dbCol, firstVal)}})

		case FilterOpGreaterOrEqual:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$gte": coerceValue(dbCol, firstVal)}})

		case FilterOpLessThan:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$lt": coerceValue(dbCol, firstVal)}})

		case FilterOpLessOrEqual:
			clauses = append(clauses, bson.M{dbCol: bson.M{"$lte": coerceValue(dbCol, firstVal)}})

		case FilterOpBetween:
			if len(f.Value) >= 2 {
				clauses = append(clauses, bson.M{
					dbCol: bson.M{
						"$gte": coerceValue(dbCol, f.Value[0]),
						"$lte": coerceValue(dbCol, f.Value[1]),
					},
				})
			}
		}
	}
	return clauses
}

func coerceValue(col string, val any) any {
	if val == nil {
		return nil
	}
	strVal, isStr := val.(string)
	if isStr {
		strVal = strings.TrimSpace(strVal)
		if len(strVal) == 24 && (strings.HasSuffix(col, "_id") || strings.HasSuffix(col, "Id") || col == "id") {
			if isHex24(strVal) {
				if oid, err := primitive.ObjectIDFromHex(strVal); err == nil {
					return oid
				}
			}
		}
		if looksLikeDate(strVal) {
			if t, err := time.Parse(time.RFC3339Nano, strVal); err == nil {
				return t
			}
			if t, err := time.Parse(time.RFC3339, strVal); err == nil {
				return t
			}
			if t, err := time.Parse("2006-01-02", strVal); err == nil {
				return t
			}
		}
	}
	return val
}

func isHex24(s string) bool {
	if len(s) != 24 {
		return false
	}
	for i := 0; i < 24; i++ {
		b := s[i]
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}

func looksLikeDate(s string) bool {
	if len(s) < 10 {
		return false
	}
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if s[5] < '0' || s[5] > '9' || s[6] < '0' || s[6] > '9' {
		return false
	}
	if s[8] < '0' || s[8] > '9' || s[9] < '0' || s[9] > '9' {
		return false
	}
	return true
}

func coerceSlice(col string, val any) []any {
	if val == nil {
		return nil
	}
	var result []any
	switch s := val.(type) {
	case []any:
		result = make([]any, 0, len(s))
		for _, item := range s {
			result = append(result, coerceValue(col, item))
		}
	case []string:
		result = make([]any, 0, len(s))
		for _, item := range s {
			result = append(result, coerceValue(col, item))
		}
	default:
		result = []any{coerceValue(col, val)}
	}
	return result
}
