package table

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// RowIterator provides a database-agnostic interface for streaming rows into exports.
// Implementations exist for in-memory slices, MongoDB cursors, and SQL/pgx row scanners.
type RowIterator[T any] interface {
	Next(ctx context.Context) bool
	Item() (T, error)
	Close(ctx context.Context) error
}

// SliceIterator wraps any Go slice into a RowIterator.
type SliceIterator[T any] struct {
	items []T
	index int
}

// NewSliceIterator creates a RowIterator for any standard in-memory slice.
func NewSliceIterator[T any](items []T) *SliceIterator[T] {
	return &SliceIterator[T]{items: items, index: -1}
}

func (s *SliceIterator[T]) Next(ctx context.Context) bool {
	s.index++
	return s.index < len(s.items)
}

func (s *SliceIterator[T]) Item() (T, error) {
	if s.index >= 0 && s.index < len(s.items) {
		return s.items[s.index], nil
	}
	var zero T
	return zero, fmt.Errorf("out of bounds")
}

func (s *SliceIterator[T]) Close(ctx context.Context) error {
	return nil
}

// MongoCursorIterator wraps a MongoDB cursor into a database-agnostic RowIterator.
type MongoCursorIterator[T any] struct {
	cursor *mongo.Cursor
}

// NewMongoCursorIterator creates a RowIterator from a MongoDB cursor.
func NewMongoCursorIterator[T any](cursor *mongo.Cursor) *MongoCursorIterator[T] {
	return &MongoCursorIterator[T]{cursor: cursor}
}

func (m *MongoCursorIterator[T]) Next(ctx context.Context) bool {
	return m.cursor.Next(ctx)
}

func (m *MongoCursorIterator[T]) Item() (T, error) {
	var item T
	err := m.cursor.Decode(&item)
	return item, err
}

func (m *MongoCursorIterator[T]) Close(ctx context.Context) error {
	return m.cursor.Close(ctx)
}

// StreamExport streams tabular data from any database RowIterator into the HTTP response.
// Supports both CSV (with UTF-8 BOM) and Microsoft Excel SpreadsheetML.
// Achieves O(1) memory overhead and O(1) Time-To-First-Byte (TTFB).
func StreamExport[T any](
	c *gin.Context,
	filename string,
	iter RowIterator[T],
	columns []TableColumn,
	rowMapper func(item T) map[string][]TableCell,
	format string,
) {
	ctx := c.Request.Context()
	defer iter.Close(ctx)

	cleanName := sanitizeFilename(filename)

	headers := make([]string, 0, len(columns))
	colIDs := make([]string, 0, len(columns))
	for _, col := range columns {
		if col.Hidden {
			continue
		}
		headers = append(headers, col.Title)
		colIDs = append(colIDs, col.ID)
	}

	fmtLower := strings.ToLower(strings.TrimSpace(format))
	if fmtLower == "excel" || fmtLower == "xlsx" || fmtLower == "xls" {
		streamExcelSpreadsheetML(c, cleanName, iter, headers, colIDs, rowMapper)
		return
	}

	streamCSV(c, cleanName, iter, headers, colIDs, rowMapper)
}

// StreamExportSlice streams tabular data from an in-memory slice directly to the HTTP client.
func StreamExportSlice[T any](
	c *gin.Context,
	filename string,
	items []T,
	columns []TableColumn,
	rowMapper func(item T) map[string][]TableCell,
	format string,
) {
	iter := NewSliceIterator(items)
	StreamExport(c, filename, iter, columns, rowMapper, format)
}

// StreamExportFromCursor streams tabular data from a MongoDB cursor (maintained for backward compatibility).
func StreamExportFromCursor[T any](
	c *gin.Context,
	filename string,
	cursor *mongo.Cursor,
	columns []TableColumn,
	rowMapper func(item T) map[string][]TableCell,
	format string,
) {
	iter := NewMongoCursorIterator[T](cursor)
	StreamExport(c, filename, iter, columns, rowMapper, format)
}

func streamCSV[T any](
	c *gin.Context,
	filename string,
	iter RowIterator[T],
	headers []string,
	colIDs []string,
	rowMapper func(item T) map[string][]TableCell,
) {
	ctx := c.Request.Context()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.csv\"", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Writer.WriteHeader(http.StatusOK)

	bufWriter := bufio.NewWriterSize(c.Writer, 64*1024)

	// UTF-8 BOM so Excel opens CSV without character corruption
	_, _ = bufWriter.Write([]byte("\xEF\xBB\xBF"))

	csvWriter := csv.NewWriter(bufWriter)

	cleanHeaders := make([]string, len(headers))
	for i, h := range headers {
		cleanHeaders[i] = sanitizeCSVField(h)
	}
	_ = csvWriter.Write(cleanHeaders)
	csvWriter.Flush()
	_ = bufWriter.Flush()
	c.Writer.Flush()

	numCols := len(colIDs)
	rowValues := make([]string, numCols)
	count := 0

	for iter.Next(ctx) {
		if ctx.Err() != nil {
			return
		}

		item, err := iter.Item()
		if err != nil {
			continue
		}

		rowMap := rowMapper(item)
		for i, colID := range colIDs {
			rowValues[i] = sanitizeCSVField(extractCellStringValue(rowMap, colID))
		}
		_ = csvWriter.Write(rowValues)
		count++

		if count%500 == 0 {
			csvWriter.Flush()
			_ = bufWriter.Flush()
			c.Writer.Flush()
		}
	}

	csvWriter.Flush()
	_ = bufWriter.Flush()
	c.Writer.Flush()
}

func streamExcelSpreadsheetML[T any](
	c *gin.Context,
	filename string,
	iter RowIterator[T],
	headers []string,
	colIDs []string,
	rowMapper func(item T) map[string][]TableCell,
) {
	ctx := c.Request.Context()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.xls\"", filename))
	c.Header("Content-Type", "application/vnd.ms-excel; charset=utf-8")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Writer.WriteHeader(http.StatusOK)

	bufWriter := bufio.NewWriterSize(c.Writer, 64*1024)

	_, _ = bufWriter.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<?mso-application progid=\"Excel.Sheet\"?>\n" +
		"<Workbook xmlns=\"urn:schemas-microsoft-com:office:spreadsheet\"\n" +
		" xmlns:o=\"urn:schemas-microsoft-com:office:office\"\n" +
		" xmlns:x=\"urn:schemas-microsoft-com:office:excel\"\n" +
		" xmlns:ss=\"urn:schemas-microsoft-com:office:spreadsheet\"\n" +
		" xmlns:html=\"http://www.w3.org/TR/REC-html40\">\n" +
		"  <Worksheet ss:Name=\"Export\">\n" +
		"    <Table>\n" +
		"      <Row>\n")

	for _, h := range headers {
		bufWriter.WriteString("        <Cell><Data ss:Type=\"String\">")
		writeXMLEscaped(bufWriter, h)
		bufWriter.WriteString("</Data></Cell>\n")
	}
	bufWriter.WriteString("      </Row>\n")

	_ = bufWriter.Flush()
	c.Writer.Flush()

	count := 0
	for iter.Next(ctx) {
		if ctx.Err() != nil {
			return
		}

		item, err := iter.Item()
		if err != nil {
			continue
		}

		rowMap := rowMapper(item)
		bufWriter.WriteString("      <Row>\n")
		for _, colID := range colIDs {
			val := extractCellStringValue(rowMap, colID)
			dataType := "String"
			if isSpreadsheetNumber(val) {
				dataType = "Number"
			}
			bufWriter.WriteString("        <Cell><Data ss:Type=\"" + dataType + "\">")
			writeXMLEscaped(bufWriter, val)
			bufWriter.WriteString("</Data></Cell>\n")
		}
		bufWriter.WriteString("      </Row>\n")
		count++

		if count%500 == 0 {
			_ = bufWriter.Flush()
			c.Writer.Flush()
		}
	}

	bufWriter.WriteString("    </Table>\n" +
		"  </Worksheet>\n" +
		"</Workbook>\n")

	_ = bufWriter.Flush()
	c.Writer.Flush()
}

func sanitizeCSVField(s string) string {
	if len(s) == 0 {
		return s
	}

	first := s[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
		if isSpreadsheetNumber(s) {
			return s
		}
		return "'" + s
	}

	return s
}

func isSpreadsheetNumber(s string) bool {
	n := len(s)
	if n == 0 {
		return false
	}

	if s[0] == '+' {
		return false
	}

	start := 0
	if s[0] == '-' {
		start = 1
		if n == 1 {
			return false
		}
	}

	if s[n-1] == '.' {
		return false
	}

	if s[start] == '0' && n > start+1 && s[start+1] != '.' {
		return false
	}

	hasDot := false
	hasDigits := false
	for i := start; i < n; i++ {
		b := s[i]
		if b >= '0' && b <= '9' {
			hasDigits = true
		} else if b == '.' {
			if hasDot {
				return false
			}
			hasDot = true
		} else {
			return false
		}
	}

	return hasDigits
}

func writeXMLEscaped(w *bufio.Writer, s string) {
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch b {
		case '&':
			_, _ = w.WriteString("&amp;")
		case '<':
			_, _ = w.WriteString("&lt;")
		case '>':
			_, _ = w.WriteString("&gt;")
		case '"':
			_, _ = w.WriteString("&quot;")
		case '\'':
			_, _ = w.WriteString("&apos;")
		default:
			if (b < 0x20 && b != 0x09 && b != 0x0A && b != 0x0D) || b == 0x7F {
				continue
			}
			_ = w.WriteByte(b)
		}
	}
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "export"
	}

	name = strings.TrimSuffix(name, ".csv")
	name = strings.TrimSuffix(name, ".CSV")
	name = strings.TrimSuffix(name, ".xlsx")
	name = strings.TrimSuffix(name, ".XLSX")
	name = strings.TrimSuffix(name, ".xls")
	name = strings.TrimSuffix(name, ".XLS")

	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}

	clean := sb.String()
	if clean == "" {
		return "export"
	}
	return clean
}

func extractCellStringValue(rowMap map[string][]TableCell, colID string) string {
	cells, exists := rowMap[colID]
	if !exists || len(cells) == 0 {
		return ""
	}

	if len(cells) == 1 {
		t := strings.TrimSpace(cells[0].Title)
		if t != "" {
			return t
		}
		return cells[0].Label
	}

	var sb strings.Builder
	for _, cell := range cells {
		t := strings.TrimSpace(cell.Title)
		val := t
		if val == "" {
			val = cell.Label
		}
		if val != "" {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(val)
		}
	}

	return sb.String()
}
