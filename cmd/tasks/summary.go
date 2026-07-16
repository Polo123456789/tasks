package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/Polo123456789/tasks/internal/application"
	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/Polo123456789/tasks/internal/ports"
	"github.com/Polo123456789/tasks/internal/tui/presenter"
	"github.com/Polo123456789/tasks/internal/tui/screens/gantt"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

const summaryMaxLines = 20
const localSummaryMaxLines = 10

type summaryKind int

const (
	summaryOverdue summaryKind = iota
	summaryToday
	summaryActive
)

type summaryGroup struct {
	kind  summaryKind
	label string
	tasks []domain.Task
}

type summaryRenderOptions struct {
	today        domain.Date
	width        int
	color        bool
	showOrigin   bool
	partial      bool
	originLabels map[string]string
	maxLines     int
}

func writeSummary(ctx context.Context, output io.Writer, service *application.Service, today domain.Date, colorMode string, partial bool) error {
	local := service.Mode == domain.ModeLocal
	tasks, listErr := service.ListTasks(ctx, ports.TaskFilter{
		IncludeDone:      local,
		IncludeCancelled: local,
		Sort:             "updated",
	})
	partial = partial || listErr != nil
	if listErr != nil {
		// A global summary remains useful when one registered origin is
		// temporarily unavailable. The visible warning prevents a partial result
		// from looking complete while keeping shell startup reliable.
		partial = true
	}
	options := summaryRenderOptions{
		today:      today,
		width:      summaryOutputWidth(output),
		color:      summaryColorEnabled(output, colorMode),
		showOrigin: service.Mode == domain.ModeGlobal,
		partial:    partial,
	}
	if !local {
		_, writeErr := io.WriteString(output, renderSummary(tasks, options))
		return writeErr
	}
	options.maxLines = localSummaryMaxLines
	summary := strings.TrimSuffix(renderSummary(tasks, options), "\n")
	usedLines := strings.Count(summary, "\n") + 1
	ganttHeight := summaryMaxLines - usedLines - 1
	text := summary
	if ganttHeight > 0 {
		chart := renderSummaryGantt(tasks, options, ganttHeight)
		text = chart + "\n\n" + summary
	}
	_, writeErr := io.WriteString(output, text+"\n")
	return writeErr
}

func renderSummaryGantt(tasks []domain.Task, options summaryRenderOptions, height int) string {
	presented := presenter.Tasks(tasks)
	for index := range presented {
		presented[index].Origin = ""
		presented[index].Source = ""
	}
	chart := gantt.View(presented, options.today.Time(), -1, options.width, height, 1)
	if !options.color {
		chart = ansi.Strip(chart)
	}
	lines := strings.Split(chart, "\n")
	for index := range lines {
		lines[index] = ansi.TruncateWc(lines[index], options.width, "…")
	}
	return strings.Join(lines, "\n")
}

func groupSummaryTasks(tasks []domain.Task, today domain.Date) []summaryGroup {
	groups := []summaryGroup{
		{kind: summaryOverdue, label: "ATRASADAS"},
		{kind: summaryToday, label: "PARA HOY"},
		{kind: summaryActive, label: "ACTIVAS"},
	}
	for _, task := range tasks {
		if task.DeletedAt != nil || task.Status.Kind == domain.StatusDone || task.Status.Kind == domain.StatusCancelled {
			continue
		}
		switch {
		case task.Due != nil && task.Due.Before(today):
			groups[0].tasks = append(groups[0].tasks, task)
		case taskBelongsToToday(task, today):
			groups[1].tasks = append(groups[1].tasks, task)
		case task.Status.Kind == domain.StatusNormal && !task.Status.Initial:
			groups[2].tasks = append(groups[2].tasks, task)
		}
	}
	sort.SliceStable(groups[0].tasks, func(i, j int) bool {
		left, right := groups[0].tasks[i], groups[0].tasks[j]
		if !left.Due.Equal(*right.Due) {
			return left.Due.Before(*right.Due)
		}
		return summaryTaskLess(left, right)
	})
	for index := 1; index < len(groups); index++ {
		sort.SliceStable(groups[index].tasks, func(i, j int) bool {
			return summaryTaskLess(groups[index].tasks[i], groups[index].tasks[j])
		})
	}
	return groups
}

func taskBelongsToToday(task domain.Task, today domain.Date) bool {
	if task.Recurrence != nil && task.RecurrenceAnchor != nil && !task.RecurrenceAnchor.After(today) {
		return true
	}
	switch {
	case task.Start != nil && task.Due != nil:
		return !today.Before(*task.Start) && !today.After(*task.Due)
	case task.Start != nil:
		return !today.Before(*task.Start)
	case task.Due != nil:
		return task.Due.Equal(today)
	default:
		return false
	}
}

func summaryTaskLess(left, right domain.Task) bool {
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	if left.Blocked != right.Blocked {
		return !left.Blocked
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	if left.Origin.Identity() != right.Origin.Identity() {
		return left.Origin.Identity() < right.Origin.Identity()
	}
	return left.ID < right.ID
}

func renderSummary(tasks []domain.Task, options summaryRenderOptions) string {
	if options.width <= 0 {
		options.width = 80
	}
	if options.maxLines <= 0 {
		options.maxLines = summaryMaxLines
	}
	options.originLabels = summaryOriginLabels(tasks)
	groups := groupSummaryTasks(tasks, options.today)
	nonempty := groups[:0]
	for _, group := range groups {
		if len(group.tasks) > 0 {
			nonempty = append(nonempty, group)
		}
	}
	groups = nonempty

	lines := []string{summaryStyled(summaryHeader(options.today), summaryHeaderStyle, options.color)}
	if len(groups) == 0 {
		lines = append(lines, summaryStyled("  ✓ Nada pendiente para hoy", summarySuccessStyle, options.color))
	} else {
		warningLines := 0
		if options.partial {
			warningLines = 1
		}
		bodyBudget := options.maxLines - len(lines) - len(groups) - warningLines
		quotas := allocateSummaryLines(groups, bodyBudget)
		layout := summaryTaskGridLayout(groups, quotas, options)
		for index, group := range groups {
			lines = append(lines, summaryStyled(fmt.Sprintf("%s · %d", group.label, len(group.tasks)), summaryGroupStyle(group.kind), options.color))
			quota := quotas[index]
			shown := quota
			showRemainder := quota >= 2 && quota < len(group.tasks)
			if showRemainder {
				shown--
			}
			for taskIndex := 0; taskIndex < shown; taskIndex++ {
				line := summaryTaskLine(group.tasks[taskIndex], group.kind, options, layout)
				lines = append(lines, summaryStyled(line, summaryTaskStyle(group.kind), options.color))
			}
			if showRemainder {
				remaining := len(group.tasks) - shown
				lines = append(lines, summaryStyled(fmt.Sprintf("  … %d más", remaining), summaryMutedStyle, options.color))
			}
		}
	}
	if options.partial {
		lines = append(lines, summaryStyled("⚠ Resumen parcial: revisa tasks.log", summaryWarningStyle, options.color))
	}
	for index := range lines {
		lines[index] = truncateSummaryLine(lines[index], options.width, options.color)
	}
	return strings.Join(lines, "\n") + "\n"
}

type summaryTaskCells struct {
	title, blocked, priority, subtasks, planning, status string
}

type summaryGridColumn struct {
	value func(summaryTaskCells) string
	width int
}

type summaryGridLayout struct {
	titleWidth int
	columns    []summaryGridColumn
}

func summaryTaskGridLayout(groups []summaryGroup, quotas []int, options summaryRenderOptions) summaryGridLayout {
	var rows []summaryTaskCells
	for index, group := range groups {
		shown := min(quotas[index], len(group.tasks))
		if quotas[index] >= 2 && quotas[index] < len(group.tasks) {
			shown--
		}
		for taskIndex := 0; taskIndex < shown; taskIndex++ {
			rows = append(rows, summaryCells(group.tasks[taskIndex], group.kind, options))
		}
	}
	if len(rows) == 0 {
		return summaryGridLayout{}
	}

	candidates := []struct {
		value func(summaryTaskCells) string
		cap   int
	}{
		{value: func(row summaryTaskCells) string { return row.blocked }, cap: 10},
		{value: func(row summaryTaskCells) string { return row.priority }, cap: 9},
		{value: func(row summaryTaskCells) string { return row.subtasks }, cap: 13},
		{value: func(row summaryTaskCells) string { return row.planning }, cap: 20},
		{value: func(row summaryTaskCells) string { return row.status }, cap: 18},
	}
	layout := summaryGridLayout{}
	fixedWidth := 4 // indent, marker and the space after it
	maxTitleWidth := 0
	for _, row := range rows {
		maxTitleWidth = max(maxTitleWidth, runewidth.StringWidth(row.title))
	}
	for _, candidate := range candidates {
		columnWidth := 0
		for _, row := range rows {
			columnWidth = max(columnWidth, runewidth.StringWidth(candidate.value(row)))
		}
		if columnWidth == 0 {
			continue
		}
		columnWidth = min(columnWidth, candidate.cap)
		layout.columns = append(layout.columns, summaryGridColumn{value: candidate.value, width: columnWidth})
		fixedWidth += 2 + columnWidth
	}
	availableTitleWidth := options.width - fixedWidth
	if availableTitleWidth < 16 {
		return summaryGridLayout{}
	}
	layout.titleWidth = min(maxTitleWidth, availableTitleWidth)
	return layout
}

func allocateSummaryLines(groups []summaryGroup, budget int) []int {
	quotas := make([]int, len(groups))
	if budget <= 0 {
		return quotas
	}
	for index := range groups {
		if budget == 0 {
			return quotas
		}
		quotas[index] = 1
		budget--
	}
	for budget > 0 {
		allocated := false
		for index := range groups {
			if budget == 0 {
				break
			}
			if quotas[index] >= len(groups[index].tasks) {
				continue
			}
			quotas[index]++
			budget--
			allocated = true
		}
		if !allocated {
			break
		}
	}
	return quotas
}

func summaryHeader(today domain.Date) string {
	weekdays := [...]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}
	months := [...]string{"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	date := today.Time()
	return fmt.Sprintf("tasks · %s %d %s", weekdays[date.Weekday()], date.Day(), months[date.Month()])
}

func summaryTaskLine(task domain.Task, kind summaryKind, options summaryRenderOptions, layout summaryGridLayout) string {
	markers := [...]string{"!", "◆", "▶"}
	cells := summaryCells(task, kind, options)
	if layout.titleWidth > 0 {
		line := "  " + markers[kind] + " " + summaryFit(cells.title, layout.titleWidth)
		for _, column := range layout.columns {
			line += "  " + summaryFit(column.value(cells), column.width)
		}
		return strings.TrimRight(line, " ")
	}
	metadata := []string{cells.blocked, cells.priority, cells.subtasks, cells.planning, cells.status}
	metadata = compactSummaryCells(metadata)
	line := fmt.Sprintf("  %s %s", markers[kind], cells.title)
	if len(metadata) > 0 {
		line += " · " + strings.Join(metadata, " · ")
	}
	return line
}

func summaryCells(task domain.Task, kind summaryKind, options summaryRenderOptions) summaryTaskCells {
	title := cleanSummaryText(task.Title)
	if options.showOrigin && task.Origin.Name != "" {
		origin := cleanSummaryText(options.originLabels[task.Origin.Identity()])
		if origin != "" {
			title = "[" + origin + "] " + title
		}
	}
	cells := summaryTaskCells{title: title}
	if task.Blocked {
		cells.blocked = "bloqueada"
	}
	if task.Priority != domain.PriorityNone {
		cells.priority = strings.ToLower(task.Priority.String())
	}
	if task.SubtaskCount > 0 {
		cells.subtasks = fmt.Sprintf("%d/%d subt.", task.SubtaskDoneCount, task.SubtaskCount)
	}
	if kind == summaryOverdue && task.Due != nil {
		cells.planning = "venció " + summaryDate(*task.Due, options.today)
	} else if task.Recurrence != nil {
		cells.planning = "recurrente"
	} else if kind == summaryActive && task.Start != nil && task.Start.After(options.today) {
		if task.Due != nil {
			cells.planning = summaryDate(*task.Start, options.today) + "–" + summaryDate(*task.Due, options.today)
		} else {
			cells.planning = "empieza " + summaryDate(*task.Start, options.today)
		}
	} else if task.Due != nil {
		if task.Due.Equal(options.today) {
			cells.planning = "vence hoy"
		} else {
			cells.planning = "vence " + summaryDate(*task.Due, options.today)
		}
	} else if task.Start != nil && !task.Start.Equal(options.today) {
		cells.planning = "desde " + summaryDate(*task.Start, options.today)
	}
	if kind == summaryActive {
		if status := cleanSummaryText(task.Status.Name); status != "" {
			cells.status = status
		}
	}
	return cells
}

func compactSummaryCells(values []string) []string {
	result := values[:0]
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func summaryFit(value string, width int) string {
	value = runewidth.Truncate(value, width, "…")
	return value + strings.Repeat(" ", max(0, width-runewidth.StringWidth(value)))
}

func summaryOriginLabels(tasks []domain.Task) map[string]string {
	labels := make(map[string]string)
	sources := make(map[string]map[string]struct{})
	for _, task := range tasks {
		if sources[task.Origin.Name] == nil {
			sources[task.Origin.Name] = make(map[string]struct{})
		}
		sources[task.Origin.Name][task.Origin.Identity()] = struct{}{}
	}
	for _, task := range tasks {
		label := task.Origin.Name
		if len(sources[label]) > 1 && task.Origin.Kind == domain.OriginProject {
			label = task.Origin.Key
		}
		labels[task.Origin.Identity()] = label
	}
	return labels
}

func summaryDate(value, today domain.Date) string {
	months := [...]string{"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	date := value.Time()
	if date.Year() != today.Time().Year() {
		return fmt.Sprintf("%d %s %d", date.Day(), months[date.Month()], date.Year())
	}
	return fmt.Sprintf("%d %s", date.Day(), months[date.Month()])
}

func cleanSummaryText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func summaryOutputWidth(output io.Writer) int {
	width := 80
	if file, ok := output.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if terminalWidth, _, err := term.GetSize(int(file.Fd())); err == nil && terminalWidth > 0 {
			width = terminalWidth
		}
	}
	return normalizeSummaryWidth(width)
}

func normalizeSummaryWidth(width int) int {
	if width < 20 {
		return 20
	}
	return width
}

func summaryColorEnabled(output io.Writer, mode string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := output.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

type summaryStyle string

const (
	summaryHeaderStyle  summaryStyle = "1;36"
	summaryDangerStyle  summaryStyle = "1;31"
	summaryTodayStyle   summaryStyle = "1;33"
	summaryActiveStyle  summaryStyle = "1;32"
	summarySuccessStyle summaryStyle = "32"
	summaryMutedStyle   summaryStyle = "2;37"
	summaryWarningStyle summaryStyle = "33"
)

func summaryGroupStyle(kind summaryKind) summaryStyle { return summaryTaskStyle(kind) }

func summaryTaskStyle(kind summaryKind) summaryStyle {
	switch kind {
	case summaryOverdue:
		return summaryDangerStyle
	case summaryToday:
		return summaryTodayStyle
	default:
		return summaryActiveStyle
	}
}

func summaryStyled(value string, style summaryStyle, enabled bool) string {
	if !enabled {
		return value
	}
	return "\x1b[" + string(style) + "m" + value + "\x1b[0m"
}

func truncateSummaryLine(line string, width int, colored bool) string {
	if !colored {
		return runewidth.Truncate(line, width, "…")
	}
	start := strings.IndexByte(line, 'm')
	end := strings.LastIndex(line, "\x1b[0m")
	if start < 0 || end < start {
		return line
	}
	prefix := line[:start+1]
	content := line[start+1 : end]
	return prefix + runewidth.Truncate(content, width, "…") + "\x1b[0m"
}
