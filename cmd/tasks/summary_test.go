package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
	"github.com/mattn/go-runewidth"
)

func mustSummaryDate(t *testing.T, value string) domain.Date {
	t.Helper()
	date, err := domain.ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func TestSummaryClassifiesWithoutDuplicatingTasks(t *testing.T) {
	today := mustSummaryDate(t, "2026-07-16")
	yesterday := today.AddDays(-1)
	tomorrow := today.AddDays(1)
	initial := domain.Status{ID: 1, Name: "Pendiente", Kind: domain.StatusNormal, Initial: true}
	active := domain.Status{ID: 2, Name: "En progreso", Kind: domain.StatusNormal}
	done := domain.Status{ID: 3, Name: "Finalizada", Kind: domain.StatusDone}
	recurrence := domain.Recurrence{Kind: domain.Weekly, Weekdays: []time.Weekday{time.Monday}}
	tasks := []domain.Task{
		{ID: 1, Title: "Vencida", Status: initial, Due: &yesterday, Priority: domain.PriorityHigh},
		{ID: 2, Title: "Intervalo actual", Status: active, Start: &yesterday, Due: &tomorrow},
		{ID: 3, Title: "Vence hoy", Status: initial, Due: &today},
		{ID: 4, Title: "Ciclo pendiente", Status: initial, Recurrence: &recurrence, RecurrenceAnchor: &yesterday},
		{ID: 5, Title: "Activa sin fecha", Status: active, Priority: domain.PriorityUrgent},
		{ID: 6, Title: "Pendiente sin fecha", Status: initial},
		{ID: 7, Title: "Terminada vencida", Status: done, Due: &yesterday},
	}
	groups := groupSummaryTasks(tasks, today)
	if len(groups[0].tasks) != 1 || groups[0].tasks[0].ID != 1 {
		t.Fatalf("overdue=%#v", groups[0].tasks)
	}
	if len(groups[1].tasks) != 3 {
		t.Fatalf("today=%#v", groups[1].tasks)
	}
	gotToday := []int64{groups[1].tasks[0].ID, groups[1].tasks[1].ID, groups[1].tasks[2].ID}
	if !sameSummaryIDs(gotToday, []int64{2, 3, 4}) {
		t.Fatalf("today=%#v", groups[1].tasks)
	}
	if len(groups[2].tasks) != 1 || groups[2].tasks[0].ID != 5 {
		t.Fatalf("active=%#v", groups[2].tasks)
	}

	rendered := renderSummary(tasks, summaryRenderOptions{today: today, width: 100})
	for _, title := range []string{"Vencida", "Intervalo actual", "Vence hoy", "Ciclo pendiente", "Activa sin fecha"} {
		if strings.Count(rendered, title) != 1 {
			t.Fatalf("%q count in summary:\n%s", title, rendered)
		}
	}
	if strings.Contains(rendered, "Pendiente sin fecha") || strings.Contains(rendered, "Terminada vencida") {
		t.Fatalf("irrelevant task included:\n%s", rendered)
	}
}

func sameSummaryIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[int64]bool, len(got))
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

func TestSummaryUsesProjectContextMetadataAndSanitizesLines(t *testing.T) {
	today := mustSummaryDate(t, "2026-07-16")
	due := today.AddDays(-2)
	task := domain.Task{
		ID:       1,
		Origin:   domain.TaskOrigin{Kind: domain.OriginProject, Key: "/work/backend.tasks", Name: "backend"},
		Title:    "Arreglar\nrespaldo\tnocturno",
		Status:   domain.Status{Name: "En progreso", Kind: domain.StatusNormal},
		Priority: domain.PriorityUrgent,
		Due:      &due,
		Blocked:  true,
	}
	global := renderSummary([]domain.Task{task}, summaryRenderOptions{today: today, width: 100, showOrigin: true})
	for _, text := range []string{"[backend]", "Arreglar respaldo nocturno", "bloqueada", "urgente", "venció 14 jul"} {
		if !strings.Contains(global, text) {
			t.Fatalf("global summary missing %q:\n%s", text, global)
		}
	}
	if strings.Count(strings.TrimSuffix(global, "\n"), "\n") != 2 {
		t.Fatalf("task title injected an extra line:\n%s", global)
	}
	local := renderSummary([]domain.Task{task}, summaryRenderOptions{today: today, width: 100})
	if strings.Contains(local, "[backend]") {
		t.Fatalf("local summary showed redundant project:\n%s", local)
	}
}

func TestSummaryDisambiguatesGlobalFromSameNamedProject(t *testing.T) {
	today, _ := domain.ParseDate("2026-07-16")
	tasks := []domain.Task{
		{ID: 1, Title: "Own", Due: &today, Origin: domain.TaskOrigin{Kind: domain.OriginGlobal, Key: domain.GlobalOriginKey, Name: "Global"}},
		{ID: 1, Title: "Project", Due: &today, Origin: domain.TaskOrigin{Kind: domain.OriginProject, Key: "/work/Global.tasks", Name: "Global"}},
	}
	rendered := renderSummary(tasks, summaryRenderOptions{today: today, width: 120, showOrigin: true})
	for _, expected := range []string{"[Global] Own", "[/work/Global.tasks] Project"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("summary missing %q:\n%s", expected, rendered)
		}
	}
}

func TestSummaryNeverExceedsTwentyLinesOrRequestedWidth(t *testing.T) {
	today := mustSummaryDate(t, "2026-07-16")
	initial := domain.Status{Name: "Pendiente", Kind: domain.StatusNormal, Initial: true}
	active := domain.Status{Name: "En progreso", Kind: domain.StatusNormal}
	var tasks []domain.Task
	for index := 0; index < 90; index++ {
		task := domain.Task{ID: int64(index + 1), Title: strings.Repeat("tarea-muy-larga-", 8), Status: initial}
		switch index % 3 {
		case 0:
			due := today.AddDays(-index - 1)
			task.Due = &due
		case 1:
			start := today
			task.Start = &start
		case 2:
			task.Status = active
		}
		tasks = append(tasks, task)
	}
	for _, color := range []bool{false, true} {
		rendered := renderSummary(tasks, summaryRenderOptions{today: today, width: 48, color: color, partial: true})
		lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
		if len(lines) > summaryMaxLines {
			t.Fatalf("color=%v produced %d lines", color, len(lines))
		}
		for _, line := range lines {
			plain := summaryANSI.ReplaceAllString(line, "")
			if width := runewidth.StringWidth(plain); width > 48 {
				t.Fatalf("color=%v line width=%d: %q", color, width, plain)
			}
		}
		if !strings.Contains(rendered, "más") || !strings.Contains(rendered, "Resumen parcial") {
			t.Fatalf("truncation or warning missing:\n%s", rendered)
		}
		if color && !strings.Contains(rendered, "\x1b[") {
			t.Fatalf("forced color missing:\n%q", rendered)
		}
	}
}

var summaryANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestEmptySummaryIsCompact(t *testing.T) {
	today := mustSummaryDate(t, "2026-07-16")
	rendered := renderSummary(nil, summaryRenderOptions{today: today, width: 80})
	if rendered != "tasks · jue 16 jul\n  ✓ Nada pendiente para hoy\n" {
		t.Fatalf("empty summary=%q", rendered)
	}
}

func TestSummaryGanttUsesCurrentMonthAndHonorsPresentationOptions(t *testing.T) {
	today := mustSummaryDate(t, "2026-07-16")
	start := mustSummaryDate(t, "2026-07-14")
	due := mustSummaryDate(t, "2026-07-18")
	tasks := []domain.Task{{
		ID:     1,
		Title:  "Preparar despliegue",
		Status: domain.Status{Name: "En progreso", Kind: domain.StatusNormal},
		Start:  &start,
		Due:    &due,
	}}

	rendered := renderSummaryGantt(tasks, summaryRenderOptions{today: today, width: 48}, 6)
	for _, expected := range []string{"Gantt · julio 2026", "Preparar desp", "●"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Gantt missing %q:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("plain Gantt emitted ANSI: %q", rendered)
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) > 6 {
		t.Fatalf("Gantt produced %d lines", len(lines))
	}
	for _, line := range lines {
		if width := runewidth.StringWidth(line); width > 48 {
			t.Fatalf("Gantt line width=%d: %q", width, line)
		}
	}
}

func TestSummaryWidthUsesAllAvailableColumns(t *testing.T) {
	for _, test := range []struct {
		available int
		want      int
	}{
		{available: 10, want: 20},
		{available: 80, want: 80},
		{available: 160, want: 160},
	} {
		if got := normalizeSummaryWidth(test.available); got != test.want {
			t.Fatalf("normalizeSummaryWidth(%d)=%d, want %d", test.available, got, test.want)
		}
	}
}
