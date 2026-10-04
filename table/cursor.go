package table

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CursorToken represents the payload encoded inside a URL-safe base64 keyset cursor string.
// Compatible with any database identifier (MongoDB ObjectID, PostgreSQL UUID, MySQL auto-increment ID).
type CursorToken struct {
	ID        string `json:"i"`           // String representation of ID (hex ObjectID, UUID, int)
	SortField string `json:"f,omitempty"` // The database field used for sorting
	SortValue any    `json:"v,omitempty"` // The value of the sort field for boundary comparison
}

// EncodeCursorGeneric builds a URL-safe base64 cursor token from any ID string (hex ObjectID, UUID, integer) and optional sort value.
func EncodeCursorGeneric(id string, sortField string, sortVal any) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	token := CursorToken{
		ID:        id,
		SortField: sortField,
		SortValue: sortVal,
	}
	bytes, err := json.Marshal(token)
	if err != nil {
		return id
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

// DecodeCursorGeneric parses an opaque cursor string back into an ID string, sort field, and sort value.
func DecodeCursorGeneric(cursorStr string) (id string, sortField string, sortVal any, err error) {
	cursorStr = strings.TrimSpace(cursorStr)
	if cursorStr == "" {
		return "", "", nil, fmt.Errorf("empty cursor")
	}

	decodedBytes, bErr := base64.RawURLEncoding.DecodeString(cursorStr)
	if bErr == nil {
		var token CursorToken
		if jErr := json.Unmarshal(decodedBytes, &token); jErr == nil && token.ID != "" {
			sortVal := token.SortValue
			if sVal, ok := sortVal.(string); ok && sVal != "" {
				if t, pErr := time.Parse(time.RFC3339Nano, sVal); pErr == nil {
					sortVal = t
				} else if t, pErr := time.Parse(time.RFC3339, sVal); pErr == nil {
					sortVal = t
				}
			}
			return token.ID, token.SortField, sortVal, nil
		}
	}

	// Fallback to raw string ID
	return cursorStr, "", nil, nil
}

// EncodeCursor builds a URL-safe base64 cursor token from a MongoDB ObjectID and optional sort value.
func EncodeCursor(id primitive.ObjectID, sortField string, sortVal any) string {
	if id.IsZero() {
		return ""
	}
	return EncodeCursorGeneric(id.Hex(), sortField, sortVal)
}

// DecodeCursor parses an opaque cursor string back into an ObjectID, sort field, and sort value.
func DecodeCursor(cursorStr string) (id primitive.ObjectID, sortField string, sortVal any, err error) {
	idStr, sField, sVal, dErr := DecodeCursorGeneric(cursorStr)
	if dErr != nil {
		return primitive.NilObjectID, "", nil, dErr
	}

	if objID, oErr := primitive.ObjectIDFromHex(idStr); oErr == nil {
		return objID, sField, sVal, nil
	}

	return primitive.NilObjectID, "", nil, fmt.Errorf("cursor id is not a valid 24-hex ObjectID")
}

// BuildCursorFilter constructs a MongoDB keyset filter for forward infinite scroll.
func BuildCursorFilter(cursorID primitive.ObjectID, sortField string, sortVal any, sortOrder int) bson.M {
	if sortField == "" || sortField == "_id" || sortVal == nil {
		if sortOrder == -1 {
			return bson.M{"_id": bson.M{"$lt": cursorID}}
		}
		return bson.M{"_id": bson.M{"$gt": cursorID}}
	}

	if sortOrder == -1 {
		if sortVal != nil {
			return bson.M{
				"$or": []bson.M{
					{sortField: bson.M{"$lt": sortVal}},
					{sortField: sortVal, "_id": bson.M{"$lt": cursorID}},
				},
			}
		}
		return bson.M{"_id": bson.M{"$lt": cursorID}}
	}

	if sortVal != nil {
		return bson.M{
			"$or": []bson.M{
				{sortField: bson.M{"$gt": sortVal}},
				{sortField: sortVal, "_id": bson.M{"$gt": cursorID}},
			},
		}
	}
	return bson.M{"_id": bson.M{"$gt": cursorID}}
}

// ExtractItemCursor extracts the ObjectID and sort value from any struct instance.
func ExtractItemCursor(item any, sortField string) (primitive.ObjectID, any) {
	idStr, sortVal := ExtractItemCursorGeneric(item, sortField)
	if idStr != "" {
		if oid, err := primitive.ObjectIDFromHex(idStr); err == nil {
			return oid, sortVal
		}
	}
	return primitive.NilObjectID, nil
}

// CursorProvider is an optional interface structs can implement to bypass reflection entirely.
type CursorProvider interface {
	CursorID() string
	CursorSortValue(sortField string) any
}

type structCursorMeta struct {
	idIndex    int
	fieldByTag map[string]int
}

var structCursorCache sync.Map // map[reflect.Type]*structCursorMeta

func getStructCursorMeta(t reflect.Type) *structCursorMeta {
	if cached, ok := structCursorCache.Load(t); ok {
		return cached.(*structCursorMeta)
	}

	meta := &structCursorMeta{
		idIndex:    -1,
		fieldByTag: make(map[string]int, t.NumField()*4),
	}

	// 1. Locate ID field
	if idField, ok := t.FieldByName("ID"); ok {
		meta.idIndex = idField.Index[0]
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tagBson := field.Tag.Get("bson")
		tagGorm := field.Tag.Get("gorm")
		tagDb := field.Tag.Get("db")
		tagJson := field.Tag.Get("json")

		if meta.idIndex == -1 {
			if strings.HasPrefix(tagBson, "_id") ||
				strings.Contains(tagGorm, "primaryKey") ||
				strings.HasPrefix(tagDb, "id") ||
				strings.HasPrefix(tagJson, "id") {
				meta.idIndex = i
			}
		}

		bsonTag := strings.Split(tagBson, ",")[0]
		if bsonTag != "" && bsonTag != "-" {
			meta.fieldByTag[bsonTag] = i
		}
		jsonTag := strings.Split(tagJson, ",")[0]
		if jsonTag != "" && jsonTag != "-" {
			meta.fieldByTag[jsonTag] = i
		}
		dbTag := strings.Split(tagDb, ",")[0]
		if dbTag != "" && dbTag != "-" {
			meta.fieldByTag[dbTag] = i
		}
		meta.fieldByTag[field.Name] = i
		meta.fieldByTag[strings.ToLower(field.Name)] = i
	}

	structCursorCache.Store(t, meta)
	return meta
}

// ExtractItemCursorGeneric extracts string ID and sort value from any struct instance (PostgreSQL, MySQL, SQLite, MongoDB).
func ExtractItemCursorGeneric(item any, sortField string) (string, any) {
	if item == nil {
		return "", nil
	}

	// 1. Zero-reflection fast path if type implements CursorProvider
	if cp, ok := item.(CursorProvider); ok {
		return cp.CursorID(), cp.CursorSortValue(sortField)
	}

	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return "", nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return "", nil
	}

	meta := getStructCursorMeta(val.Type())

	var foundID string
	var foundSortVal any

	// 1. Get cached ID field
	if meta.idIndex >= 0 {
		foundID = formatReflectValue(val.Field(meta.idIndex).Interface())
	}

	// 2. Locate sort field value via cached field indices
	if sortField != "" && sortField != "_id" && sortField != "id" {
		idx, ok := meta.fieldByTag[sortField]
		if !ok {
			idx, ok = meta.fieldByTag[strings.ToLower(sortField)]
		}
		if ok {
			foundVal := val.Field(idx).Interface()
			switch t := foundVal.(type) {
			case time.Time:
				foundSortVal = t.Format(time.RFC3339Nano)
			case primitive.DateTime:
				foundSortVal = t.Time().Format(time.RFC3339Nano)
			default:
				foundSortVal = foundVal
			}
		}
	}

	return foundID, foundSortVal
}

func formatReflectValue(v any) string {
	if v == nil {
		return ""
	}
	if oid, ok := v.(primitive.ObjectID); ok {
		if oid.IsZero() {
			return ""
		}
		return oid.Hex()
	}
	if s, ok := v.(string); ok {
		return s
	}
	if stringer, ok := v.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%v", v)
}
