package table

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BenchmarkItem struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Email     string             `bson:"email" json:"email"`
	Status    string             `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type BenchmarkItemProvider struct {
	ID        primitive.ObjectID
	CreatedAt time.Time
}

func (b BenchmarkItemProvider) CursorID() string {
	return b.ID.Hex()
}

func (b BenchmarkItemProvider) CursorSortValue(sortField string) any {
	if sortField == "created_at" {
		return b.CreatedAt.Format(time.RFC3339Nano)
	}
	return nil
}

func BenchmarkExtractItemCursor_CursorProvider(b *testing.B) {
	item := BenchmarkItemProvider{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id, sortVal := ExtractItemCursorGeneric(item, "created_at")
		if id == "" || sortVal == nil {
			b.Fatal("failed to extract cursor")
		}
	}
}

func BenchmarkExtractItemCursorGeneric(b *testing.B) {
	item := BenchmarkItem{
		ID:        primitive.NewObjectID(),
		Name:      "Test User",
		Email:     "test@example.com",
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id, sortVal := ExtractItemCursorGeneric(item, "created_at")
		if id == "" || sortVal == nil {
			b.Fatal("failed to extract cursor")
		}
	}
}

func BenchmarkCoerceValue_NonDate(b *testing.B) {
	val := "ACTIVE_STATUS"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res := coerceValue("status", val)
		if res != val {
			b.Fatal("mismatch")
		}
	}
}

func BenchmarkCoerceValue_Date(b *testing.B) {
	val := "2026-10-04T12:00:00Z"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res := coerceValue("created_at", val)
		if _, ok := res.(time.Time); !ok {
			b.Fatal("expected time.Time")
		}
	}
}

func BenchmarkExtractCellStringValue(b *testing.B) {
	rowMap := map[string][]TableCell{
		"name":   {NewTextCell("John Doe", "")},
		"status": {Badge("Active", BadgeColors["SUCCESS"])},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := extractCellStringValue(rowMap, "name")
		if s != "John Doe" {
			b.Fatal("mismatch")
		}
	}
}
