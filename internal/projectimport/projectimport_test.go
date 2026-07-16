package projectimport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Polo123456789/tasks/internal/domain"
)

func date(t *testing.T, value string) domain.Date {
	t.Helper()
	parsed, err := domain.ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func stringPointer(value string) *string { return &value }

func validDocument() Document {
	return Document{
		Format:  Format,
		Version: Version,
		Statuses: []StatusSpec{
			{Key: "todo", Name: "Por hacer", Initial: true},
			{Key: "doing", Name: "En curso"},
		},
		Tasks: []TaskSpec{
			{Key: "first", Title: "Primera", Status: StatusDone, Priority: "high", Due: stringPointer("2026-07-20")},
			{Key: "second", Title: "Segunda", Markdown: "# Contexto", Start: stringPointer("2026-07-21"), Due: stringPointer("2026-07-22"), Subtasks: []SubtaskSpec{{Title: "Una"}}, DependsOn: []string{"first"}},
		},
	}
}

func TestDecodeIsStrict(t *testing.T) {
	valid := `{"format":"tasks-project","version":1,"statuses":[{"key":"todo","name":"Pendiente","initial":true}],"tasks":[]}`
	document, err := Decode(strings.NewReader(valid))
	if err != nil || document.Version != 1 {
		t.Fatalf("document=%#v err=%v", document, err)
	}
	for name, input := range map[string]string{
		"unknown":  `{"format":"tasks-project","version":1,"statuses":[],"tasks":[],"extra":true}`,
		"trailing": valid + ` {}`,
		"fenced":   "```json\n" + valid + "\n```",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(input)); err == nil {
				t.Fatal("expected strict decoding error")
			}
		})
	}
}

func TestNormalizeCompleteDocumentAndDefaults(t *testing.T) {
	today := date(t, "2026-07-16")
	seed, err := Normalize(validDocument(), today)
	if err != nil {
		t.Fatal(err)
	}
	if len(seed.Statuses) != 2 || len(seed.Tasks) != 2 {
		t.Fatalf("seed=%#v", seed)
	}
	second := seed.Tasks[1]
	if second.StatusKey != "todo" || second.Task.Priority != domain.PriorityNone || second.Task.Markdown != "# Contexto" {
		t.Fatalf("defaults=%#v", second)
	}
	if second.Task.Start.String() != "2026-07-21" || second.Task.Due.String() != "2026-07-22" {
		t.Fatalf("dates=%v..%v", second.Task.Start, second.Task.Due)
	}
	if len(second.Subtasks) != 1 || second.Subtasks[0].StatusKey != "todo" || second.DependsOn[0] != "first" {
		t.Fatalf("relations=%#v", second)
	}
}

func TestNormalizeRecurrenceAndLifecycleRules(t *testing.T) {
	document := validDocument()
	document.Tasks = []TaskSpec{
		{Key: "recurring", Title: "Rutina", Recurrence: stringPointer("weekly:mon,thu")},
		{Key: "completed", Title: "Lista", Subtasks: []SubtaskSpec{{Title: "A", Status: StatusDone}, {Title: "B", Status: StatusDone}}},
		{Key: "cancelled", Title: "Cancelada", Status: StatusCancelled, Subtasks: []SubtaskSpec{{Title: "C", Status: "doing"}}},
	}
	today := date(t, "2026-07-16")
	seed, err := Normalize(document, today)
	if err != nil {
		t.Fatal(err)
	}
	if seed.Tasks[0].Task.Recurrence == nil || !seed.Tasks[0].Task.RecurrenceAnchor.Equal(today) {
		t.Fatalf("recurrence=%#v anchor=%v", seed.Tasks[0].Task.Recurrence, seed.Tasks[0].Task.RecurrenceAnchor)
	}
	if seed.Tasks[1].StatusKey != StatusDone {
		t.Fatalf("completed parent status=%q", seed.Tasks[1].StatusKey)
	}
	if seed.Tasks[2].Subtasks[0].StatusKey != StatusCancelled {
		t.Fatalf("cancelled child status=%q", seed.Tasks[2].Subtasks[0].StatusKey)
	}
}

func TestNormalizeRejectsInvalidReferencesAndCycles(t *testing.T) {
	today := date(t, "2026-07-16")
	tests := map[string]func(*Document){
		"unknown status": func(document *Document) { document.Tasks[0].Status = "missing" },
		"unknown task":   func(document *Document) { document.Tasks[1].DependsOn = []string{"missing"} },
		"duplicate key":  func(document *Document) { document.Tasks[1].Key = document.Tasks[0].Key },
		"self dependency": func(document *Document) {
			document.Tasks[0].DependsOn = []string{document.Tasks[0].Key}
		},
		"cycle": func(document *Document) {
			document.Tasks[0].DependsOn = []string{"second"}
			document.Tasks[1].DependsOn = []string{"first"}
		},
		"invalid dates": func(document *Document) {
			document.Tasks[0].Start = stringPointer("2026-07-22")
			document.Tasks[0].Due = stringPointer("2026-07-20")
		},
		"dates and recurrence": func(document *Document) { document.Tasks[0].Recurrence = stringPointer("daily") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			document := validDocument()
			mutate(&document)
			_, err := Normalize(document, today)
			if err == nil || !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestPromptContainsCurrentContract(t *testing.T) {
	today := date(t, "2026-07-16")
	prompt := Prompt(today)
	for _, expected := range []string{"2026-07-16", `"tasks-project"`, "Responde únicamente con JSON puro", "depends_on", "monthly-weekday:last:fri"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
	if _, err := time.Parse("2006-01-02", today.String()); err != nil {
		t.Fatal(err)
	}
}
