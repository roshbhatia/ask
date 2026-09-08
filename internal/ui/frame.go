package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/roshbhatia/go-utils/cell"
	"github.com/roshbhatia/go-utils/terminal"
	shared "github.com/roshbhatia/go-utils/ui"
)

const (
	narrowest = 34
	widest    = 84
)

var (
	slots   = scheme()
	palette = shared.TerminalPalette()

	dim    = paint(slots, "base04", string(palette.Muted))
	accent = paint(slots, "base0D", string(palette.Accent))
	bad    = paint(slots, "base08", string(palette.Danger))
	tool   = paint(slots, "base0B", string(palette.Success))
	edge   = paint(slots, "base02", string(palette.Border))
	chosen = accent.Bold(true)
)

// inner answers with the room a frame leaves for content, given the whole terminal.
func inner(terminal int) int {
	if terminal <= 0 {
		terminal = narrowest + 4
	}
	if terminal > widest {
		terminal = widest
	}
	room := terminal - 4
	if room < narrowest {
		room = narrowest
	}
	return room
}

// split lays a left half and a right half against the two ends of one row.
func split(left, right string, width int) string {
	if width < 1 {
		return ""
	}
	if cell.Width(right) >= width {
		return cell.RightFit(right, width)
	}
	left = cell.Truncate(left, width-cell.Width(right)-1)
	gap := width - cell.Width(left) - cell.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

type frame struct {
	title string
	width int
	head  string
	rows  []string
}

func (f frame) String() string {
	border := lipgloss.RoundedBorder()
	across := f.width + 2

	var out strings.Builder
	out.WriteString(edge.Render(border.TopLeft+" ") + accent.Render(f.title) + edge.Render(" "+strings.Repeat(border.Top, max(across-2-cell.Width(f.title), 0))+border.TopRight))
	out.WriteString("\n")

	write := func(row string) {
		out.WriteString(edge.Render(border.Left) + " " + cell.Fit(row, f.width) + " " + edge.Render(border.Right) + "\n")
	}
	if f.head != "" {
		write(f.head)
		if len(f.rows) > 0 {
			out.WriteString(edge.Render(border.MiddleLeft+strings.Repeat(border.Top, across)+border.MiddleRight) + "\n")
		}
	}
	for _, row := range f.rows {
		write(row)
	}
	out.WriteString(edge.Render(border.BottomLeft + strings.Repeat(border.Top, across) + border.BottomRight))
	return out.String()
}

// console opens the terminal itself, because stdin is often a pipe holding the input.
func console() *os.File {
	handle, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	if !terminal.IsTTY(handle) {
		_ = handle.Close()
		return nil
	}
	return handle
}
