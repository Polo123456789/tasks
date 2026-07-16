package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	d, e := ParseDate("2024-02-29")
	if e != nil || d.String() != "2024-02-29" {
		t.Fatal(d, e)
	}
	if _, e = ParseDate("2023-02-29"); e == nil {
		t.Fatal("expected invalid leap day")
	}
	if d.AddDays(1).String() != "2024-03-01" {
		t.Fatal(d.AddDays(1))
	}
}
func TestValidateTask(t *testing.T) {
	start, _ := ParseDate("2025-02-02")
	due, _ := ParseDate("2025-02-01")
	e := ValidateTask(Task{Title: "x", Start: &start, Due: &due})
	if !errors.Is(e, ErrValidation) {
		t.Fatal(e)
	}
}
func TestRecurrence(t *testing.T) {
	cases := []struct {
		r          Recurrence
		from, want string
	}{{Recurrence{Kind: Daily}, "2024-12-31", "2025-01-01"}, {Recurrence{Kind: MonthlyLastDay}, "2024-01-31", "2024-02-29"}, {Recurrence{Kind: MonthlyWeekday, Ordinal: -1, Weekday: time.Friday}, "2024-02-01", "2024-02-23"}, {Recurrence{Kind: Weekly, Weekdays: []time.Weekday{time.Monday, time.Thursday}}, "2024-01-04", "2024-01-08"}}
	for _, c := range cases {
		d, _ := ParseDate(c.from)
		got, e := c.r.Next(d)
		if e != nil || got.String() != c.want {
			t.Errorf("%s: got %s %v want %s", c.from, got, e, c.want)
		}
	}
}
func FuzzRecurrenceAlwaysAdvances(f *testing.F) {
	f.Add(2024, 1, 1)
	f.Fuzz(func(t *testing.T, y, m, d int) {
		if y < 1 || y > 9998 || m < 1 || m > 12 || d < 1 || d > 28 {
			return
		}
		date, e := NewDate(y, time.Month(m), d)
		if e != nil {
			return
		}
		next, e := (Recurrence{Kind: Daily}).Next(date)
		if e != nil || !next.After(date) {
			t.Fatalf("%v %v", next, e)
		}
	})
}
