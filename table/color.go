package table

// BadgeColor holds background, text, and border hex colors.
type BadgeColor struct {
	Bg     string
	Text   string
	Border string
}

// BadgeColors provides a map of unique color groups for status badges.
var BadgeColors = map[string]BadgeColor{
	"SUCCESS":  {Bg: "#dcfce7", Text: "#166534", Border: "#166534"},
	"PENDING":  {Bg: "#fef3c7", Text: "#92400e", Border: "#92400e"},
	"REJECTED": {Bg: "#fee2e2", Text: "#991b1b", Border: "#991b1b"},
	"INFO":     {Bg: "#dbeafe", Text: "#1e40af", Border: "#1e40af"},
	"NEUTRAL":  {Bg: "#f3f4f6", Text: "#374151", Border: "#374151"},
	"SPECIAL":  {Bg: "#f3e8ff", Text: "#6b21a8", Border: "#6b21a8"},
}

// Badge creates a status cell with a custom title and BadgeColor.
func Badge(title string, color BadgeColor) TableCell {
	return TableCell{
		Type:        "STATUS",
		Title:       title,
		BgColor:     color.Bg,
		TextColor:   color.Text,
		BorderColor: color.Border,
	}
}
