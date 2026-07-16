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
	e = ValidateTask(Task{Title: "x", Recurrence: &Recurrence{Kind: Weekly}})
	if !errors.Is(e, ErrValidation) {
		t.Fatalf("invalid recurrence must be a validation error: %v", e)
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

func TestMonthlyRecurrenceCalendarEdges(t *testing.T) {
	cases := []struct {
		name       string
		r          Recurrence
		from, want string
	}{
		{"day skips short February", Recurrence{Kind: MonthlyDay, Day: 31}, "2023-01-31", "2023-03-31"},
		{"day leap February", Recurrence{Kind: MonthlyDay, Day: 29}, "2024-01-30", "2024-02-29"},
		{"day skips non-leap February", Recurrence{Kind: MonthlyDay, Day: 29}, "2023-01-30", "2023-03-29"},
		{"last day 30", Recurrence{Kind: MonthlyLastDay}, "2024-04-01", "2024-04-30"},
		{"last day year crossing", Recurrence{Kind: MonthlyLastDay}, "2024-12-31", "2025-01-31"},
		{"first Monday", Recurrence{Kind: MonthlyWeekday, Ordinal: 1, Weekday: time.Monday}, "2024-01-01", "2024-02-05"},
		{"second Tuesday", Recurrence{Kind: MonthlyWeekday, Ordinal: 2, Weekday: time.Tuesday}, "2024-02-01", "2024-02-13"},
		{"third Wednesday", Recurrence{Kind: MonthlyWeekday, Ordinal: 3, Weekday: time.Wednesday}, "2024-02-01", "2024-02-21"},
		{"fourth Thursday", Recurrence{Kind: MonthlyWeekday, Ordinal: 4, Weekday: time.Thursday}, "2024-02-01", "2024-02-22"},
		{"last Sunday", Recurrence{Kind: MonthlyWeekday, Ordinal: -1, Weekday: time.Sunday}, "2024-02-01", "2024-02-25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, _ := ParseDate(tc.from)
			got, err := tc.r.Next(from)
			if err != nil || got.String() != tc.want {
				t.Fatalf("got %s (%v), want %s", got, err, tc.want)
			}
		})
	}
}

func TestRecurrenceValidation(t *testing.T) {
	invalid := []Recurrence{
		{},
		{Kind: Weekly},
		{Kind: Weekly, Weekdays: []time.Weekday{-1}},
		{Kind: MonthlyDay, Day: 0},
		{Kind: MonthlyDay, Day: 32},
		{Kind: MonthlyWeekday, Ordinal: 0, Weekday: time.Monday},
		{Kind: MonthlyWeekday, Ordinal: 5, Weekday: time.Monday},
		{Kind: MonthlyWeekday, Ordinal: 1, Weekday: 7},
	}
	for _, recurrence := range invalid {
		if err := recurrence.Validate(); err == nil {
			t.Errorf("expected invalid recurrence: %#v", recurrence)
		}
	}
}

func TestRecurrenceTextRoundTrip(t *testing.T) {
	values := []string{
		"daily", "weekly:mon,thu", "monthly:15", "month-end",
		"monthly-weekday:first:mon", "monthly-weekday:second:tue",
		"monthly-weekday:third:wed", "monthly-weekday:fourth:thu",
		"monthly-weekday:last:fri",
	}
	for _, value := range values {
		recurrence, err := ParseRecurrence(value)
		if err != nil {
			t.Fatalf("parse %q: %v", value, err)
		}
		if got := recurrence.Text(); got != value {
			t.Errorf("round trip %q -> %q", value, got)
		}
	}
	for _, invalid := range []string{"", "cron:* * * * *", "weekly:", "weekly:foo", "monthly:0", "monthly:32", "monthly-weekday:fifth:mon", "monthly-weekday:first:nope"} {
		if _, err := ParseRecurrence(invalid); err == nil {
			t.Errorf("invalid recurrence accepted: %q", invalid)
		}
	}
}
func FuzzRecurrenceAlwaysAdvances(f *testing.F) {
	f.Add(2024, 1, 1, 0, 1, 1)
	f.Fuzz(func(t *testing.T, y, m, d, kind, value, weekday int) {
		if y < 1 || y > 9996 || m < 1 || m > 12 || d < 1 || d > 28 {
			return
		}
		date, e := NewDate(y, time.Month(m), d)
		if e != nil {
			return
		}
		var recurrence Recurrence
		switch ((kind % 5) + 5) % 5 {
		case 0:
			recurrence = Recurrence{Kind: Daily}
		case 1:
			recurrence = Recurrence{Kind: Weekly, Weekdays: []time.Weekday{time.Weekday(((weekday % 7) + 7) % 7)}}
		case 2:
			recurrence = Recurrence{Kind: MonthlyDay, Day: ((value%31)+31)%31 + 1}
		case 3:
			recurrence = Recurrence{Kind: MonthlyLastDay}
		case 4:
			ordinal := ((value%5)+5)%5 + 1
			if ordinal == 5 {
				ordinal = -1
			}
			recurrence = Recurrence{Kind: MonthlyWeekday, Ordinal: ordinal, Weekday: time.Weekday(((weekday % 7) + 7) % 7)}
		}
		next, e := recurrence.Next(date)
		if e != nil || !next.After(date) {
			t.Fatalf("%#v from %v produced %v: %v", recurrence, date, next, e)
		}
	})
}
