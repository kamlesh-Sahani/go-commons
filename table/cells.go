package table

import (
	"fmt"
	"time"
)

// TableCell represents a single interactive or formatted cell in a table row.
type TableCell struct {
	Type           string      `json:"type"`
	Title          string      `json:"title,omitempty"`
	URL            string      `json:"url,omitempty"`
	Color          string      `json:"color,omitempty"`
	IsBold         bool        `json:"isBold,omitempty"`
	Src            string      `json:"src,omitempty"`
	IsPreview      bool        `json:"isPreview,omitempty"`
	Shape          string      `json:"shape,omitempty"`
	Width          int         `json:"width,omitempty"`
	BgColor        string      `json:"bgColor,omitempty"`
	TextColor      string      `json:"textColor,omitempty"`
	BorderColor    string      `json:"borderColor,omitempty"`
	ActionKey      string      `json:"actionKey,omitempty"`
	ActionType     string      `json:"actionType,omitempty"`
	ConfirmMessage string      `json:"confirmMessage,omitempty"`
	ApiData        *ActionData `json:"actionData,omitempty"`
	ModalType      string      `json:"modalType,omitempty"`
	Label          string      `json:"label,omitempty"`
	Data           any         `json:"data,omitempty"`
}

// ActionData defines arbitrary parameters passed to frontend action handlers.
type ActionData map[string]any

// NewLinkCell creates a clickable hyperlink cell.
func NewLinkCell(title, url, color string, isBold bool) TableCell {
	return TableCell{
		Type:   "LINK",
		Title:  title,
		URL:    url,
		Color:  color,
		IsBold: isBold,
	}
}

// NewTextCell creates a standard text display cell.
func NewTextCell(title, color string, isBold ...bool) TableCell {
	bold := false
	if len(isBold) > 0 {
		bold = isBold[0]
	}
	return TableCell{
		Type:   "TEXT",
		Title:  title,
		Color:  color,
		IsBold: bold,
	}
}

// NewStatusCell creates a badge-style status cell with custom background, text, and border colors.
func NewStatusCell(title, bgColor, textColor, borderColor string) TableCell {
	return TableCell{
		Type:        "STATUS",
		Title:       title,
		BgColor:     bgColor,
		TextColor:   textColor,
		BorderColor: borderColor,
	}
}

// NewActionCell creates a row action trigger cell (e.g. edit, delete, activate).
func NewActionCell(actionKey, actionType, confirmMessage string, apiData *ActionData) TableCell {
	return TableCell{
		Type:           "ACTION",
		ActionKey:      actionKey,
		ActionType:     actionType,
		ConfirmMessage: confirmMessage,
		ApiData:        apiData,
	}
}

// NewImageCell creates an image thumbnail or icon cell.
func NewImageCell(src string, isPreview bool, shape string, width int) TableCell {
	return TableCell{
		Type:      "IMAGE",
		Src:       src,
		IsPreview: isPreview,
		Shape:     shape,
		Width:     width,
	}
}

// NewDynamicFormCell creates a form launcher cell.
func NewDynamicFormCell(label string, apiData *ActionData) TableCell {
	return TableCell{
		Type:    "DYNAMIC_FORM",
		Label:   label,
		ApiData: apiData,
	}
}

// NewModalCell creates a modal-opening cell.
func NewModalCell(modalType string, data any) TableCell {
	return TableCell{
		Type:      "MODAL",
		ModalType: modalType,
		Data:      data,
	}
}

// NewCurrencyCell creates a formatted currency amount cell.
func NewCurrencyCell(amount float64, currency string, isBold ...bool) TableCell {
	bold := false
	if len(isBold) > 0 {
		bold = isBold[0]
	}
	formatted := fmt.Sprintf("%s %.2f", currency, amount)
	return TableCell{
		Type:   "CURRENCY",
		Title:  formatted,
		IsBold: bold,
		Data: map[string]any{
			"amount":   amount,
			"currency": currency,
		},
	}
}

// NewDateCell creates a formatted date cell. Defaults layout to "02 Jan 2006".
func NewDateCell(t time.Time, format ...string) TableCell {
	layout := "02 Jan 2006"
	if len(format) > 0 && format[0] != "" {
		layout = format[0]
	}
	formatted := t.Format(layout)
	return TableCell{
		Type:  "DATE",
		Title: formatted,
		Data: map[string]any{
			"iso":       t.Format(time.RFC3339),
			"timestamp": t.Unix(),
		},
	}
}

// NewRelativeDateCell creates a relative humanized date cell (e.g. "5m ago", "2h ago", "just now").
func NewRelativeDateCell(t time.Time) TableCell {
	diff := time.Since(t)
	var rel string
	switch {
	case diff < time.Minute:
		rel = "just now"
	case diff < time.Hour:
		rel = fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		rel = fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 30*24*time.Hour:
		rel = fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		rel = t.Format("02 Jan 2006")
	}
	return TableCell{
		Type:  "DATE",
		Title: rel,
		Data: map[string]any{
			"iso":       t.Format(time.RFC3339),
			"timestamp": t.Unix(),
		},
	}
}

// NewAvatarCell creates an avatar cell with image and optional subtitle.
func NewAvatarCell(name string, src string, subtitle ...string) TableCell {
	sub := ""
	if len(subtitle) > 0 {
		sub = subtitle[0]
	}
	return TableCell{
		Type:  "AVATAR",
		Title: name,
		Src:   src,
		Data: map[string]any{
			"subtitle": sub,
		},
	}
}

// NewCopyableCell creates a text cell with a copy-to-clipboard button.
func NewCopyableCell(text string, displayText ...string) TableCell {
	title := text
	if len(displayText) > 0 && displayText[0] != "" {
		title = displayText[0]
	}
	return TableCell{
		Type:  "COPYABLE",
		Title: title,
		Data: map[string]any{
			"copyValue": text,
		},
	}
}

// NewTagCell creates a colored badge tag cell using BadgeColor.
func NewTagCell(text string, color BadgeColor) TableCell {
	return TableCell{
		Type:        "TAG",
		Title:       text,
		BgColor:     color.Bg,
		TextColor:   color.Text,
		BorderColor: color.Border,
	}
}

// NewBooleanCell creates a status badge cell representing true/false.
func NewBooleanCell(val bool, trueLabel, falseLabel string) TableCell {
	if val {
		return TableCell{
			Type:        "STATUS",
			Title:       trueLabel,
			BgColor:     BadgeColors["SUCCESS"].Bg,
			TextColor:   BadgeColors["SUCCESS"].Text,
			BorderColor: BadgeColors["SUCCESS"].Border,
		}
	}
	return TableCell{
		Type:        "STATUS",
		Title:       falseLabel,
		BgColor:     BadgeColors["NEUTRAL"].Bg,
		TextColor:   BadgeColors["NEUTRAL"].Text,
		BorderColor: BadgeColors["NEUTRAL"].Border,
	}
}

// NewProgressCell creates a percentage progress bar cell.
func NewProgressCell(percent int, status ...string) TableCell {
	st := "normal"
	if len(status) > 0 {
		st = status[0]
	}
	return TableCell{
		Type:  "PROGRESS",
		Title: fmt.Sprintf("%d%%", percent),
		Data: map[string]any{
			"percent": percent,
			"status":  st,
		},
	}
}
