package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	dateColor      lipgloss.Color = "73"
	authorColor    lipgloss.Color = "185"
	mutedColor     lipgloss.Color = "242"
	greenColor     lipgloss.Color = "114"
	redColor       lipgloss.Color = "203"
	cyanColor      lipgloss.Color = "81"
	selectionColor lipgloss.Color = "24"
)

// Render every segment with the selection background so ANSI resets between
// fields cannot leave gaps in the highlighted row.
func historyRow(date, author, revision, subject string, working, selected bool, width int) string {
	field := func(text string, color lipgloss.Color) string {
		style := lipgloss.NewStyle()
		if color != "" {
			style = style.Foreground(color)
		}
		if selected {
			style = style.Background(selectionColor)
		}
		return style.Render(text)
	}
	author = ansi.Truncate(safeText(author), 17, "…")
	author += strings.Repeat(" ", max(0, 17-ansi.StringWidth(author)))
	subjectColor := lipgloss.Color("")
	if working {
		subjectColor = redColor
	}
	line := field(date+" ", dateColor) + field(author+" ", authorColor) + field("o ", mutedColor)
	if revision != "" {
		line += field(revision+" ", greenColor)
	}
	line += field(safeText(subject), subjectColor)
	line = ansi.Truncate(line, width, "…")
	line += field(strings.Repeat(" ", max(0, width-ansi.StringWidth(line))), "")
	return line
}

func bar(text string, width int, errorState bool) string {
	bg := lipgloss.Color("24")
	if errorState {
		bg = "88"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(bg).Width(width).
		Render(ansi.Truncate(text, width, "…"))
}

func colorDiff(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		color := lipgloss.Color("")
		bold := false
		switch {
		case strings.HasPrefix(line, "Revision: "), strings.HasPrefix(line, "Author: "), strings.HasPrefix(line, "Date: "):
			label, value, _ := strings.Cut(line, ": ")
			valueColor := lipgloss.Color("")
			if label == "Revision" {
				valueColor = greenColor
			}
			if label == "Author" {
				valueColor = authorColor
			}
			lines[i] = lipgloss.NewStyle().Foreground(cyanColor).Render(label+": ") + lipgloss.NewStyle().Foreground(valueColor).Render(value)
			continue
		case strings.HasPrefix(line, "Index:"), strings.HasPrefix(line, "Property changes:"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"), line == "Changed paths:":
			color, bold = authorColor, true
		case line == "Not Committed Yet":
			color, bold = authorColor, true
		case line == "Unstaged changes", strings.HasPrefix(line, "SVN error:"), strings.Contains(line, "svn: E"):
			color = redColor
		case strings.Contains(line, "warning:"), strings.Contains(line, "svn: W"):
			color = authorColor
		case strings.HasPrefix(line, "@@"):
			color = cyanColor
		case strings.HasPrefix(line, "+"), strings.HasPrefix(line, "A "):
			color = greenColor
		case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "D "), strings.HasPrefix(line, "C "):
			color = redColor
		case strings.HasPrefix(line, "M "), strings.HasPrefix(line, "P "), strings.HasPrefix(line, "? "):
			color = authorColor
		case strings.HasPrefix(line, "===="), strings.HasPrefix(line, "____"):
			color = mutedColor
		}
		if color != "" {
			lines[i] = lipgloss.NewStyle().Foreground(color).Bold(bold).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
