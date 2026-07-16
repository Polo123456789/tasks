package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseRecurrence parses the compact recurrence syntax used by the terminal form.
// Supported forms: daily, weekly:mon,thu, monthly:15, month-end,
// and monthly-weekday:first:mon (first may be first..fourth or last).
func ParseRecurrence(value string) (Recurrence, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "daily" {
		return Recurrence{Kind: Daily}, nil
	}
	if value == "month-end" {
		return Recurrence{Kind: MonthlyLastDay}, nil
	}
	if strings.HasPrefix(value, "weekly:") {
		parts := strings.Split(strings.TrimPrefix(value, "weekly:"), ",")
		days := make([]time.Weekday, 0, len(parts))
		seen := make(map[time.Weekday]bool)
		for _, part := range parts {
			day, ok := weekdayNames[strings.TrimSpace(part)]
			if !ok {
				return Recurrence{}, fmt.Errorf("invalid weekday %q", part)
			}
			if !seen[day] {
				days = append(days, day)
				seen[day] = true
			}
		}
		recurrence := Recurrence{Kind: Weekly, Weekdays: days}
		return recurrence, recurrence.Validate()
	}
	if strings.HasPrefix(value, "monthly:") {
		day, err := strconv.Atoi(strings.TrimPrefix(value, "monthly:"))
		if err != nil {
			return Recurrence{}, fmt.Errorf("invalid month day")
		}
		recurrence := Recurrence{Kind: MonthlyDay, Day: day}
		return recurrence, recurrence.Validate()
	}
	if strings.HasPrefix(value, "monthly-weekday:") {
		parts := strings.Split(strings.TrimPrefix(value, "monthly-weekday:"), ":")
		if len(parts) != 2 {
			return Recurrence{}, fmt.Errorf("expected monthly-weekday:<ordinal>:<weekday>")
		}
		ordinals := map[string]int{"first": 1, "second": 2, "third": 3, "fourth": 4, "last": -1}
		ordinal, ok := ordinals[parts[0]]
		if !ok {
			return Recurrence{}, fmt.Errorf("invalid ordinal %q", parts[0])
		}
		weekday, ok := weekdayNames[parts[1]]
		if !ok {
			return Recurrence{}, fmt.Errorf("invalid weekday %q", parts[1])
		}
		return Recurrence{Kind: MonthlyWeekday, Ordinal: ordinal, Weekday: weekday}, nil
	}
	return Recurrence{}, fmt.Errorf("unsupported recurrence")
}

func (r Recurrence) Text() string {
	switch r.Kind {
	case Daily:
		return "daily"
	case Weekly:
		names := make([]string, 0, len(r.Weekdays))
		for _, weekday := range r.Weekdays {
			names = append(names, strings.ToLower(weekday.String()[:3]))
		}
		return "weekly:" + strings.Join(names, ",")
	case MonthlyDay:
		return fmt.Sprintf("monthly:%d", r.Day)
	case MonthlyLastDay:
		return "month-end"
	case MonthlyWeekday:
		ordinal := map[int]string{1: "first", 2: "second", 3: "third", 4: "fourth", -1: "last"}[r.Ordinal]
		return "monthly-weekday:" + ordinal + ":" + strings.ToLower(r.Weekday.String()[:3])
	default:
		return ""
	}
}

// HumanText is intended for the Spanish TUI. Text remains the stable compact
// representation used for parsing and persistence-oriented diagnostics.
func (r Recurrence) HumanText() string {
	weekdays := map[time.Weekday]string{
		time.Monday: "lunes", time.Tuesday: "martes", time.Wednesday: "miércoles",
		time.Thursday: "jueves", time.Friday: "viernes", time.Saturday: "sábado", time.Sunday: "domingo",
	}
	switch r.Kind {
	case Daily:
		return "Cada día"
	case Weekly:
		names := make([]string, 0, len(r.Weekdays))
		for _, weekday := range r.Weekdays {
			names = append(names, weekdays[weekday])
		}
		return "Cada " + strings.Join(names, " y ")
	case MonthlyDay:
		return fmt.Sprintf("Día %d de cada mes", r.Day)
	case MonthlyLastDay:
		return "Último día de cada mes"
	case MonthlyWeekday:
		ordinal := map[int]string{1: "primer", 2: "segundo", 3: "tercer", 4: "cuarto", -1: "último"}[r.Ordinal]
		return fmt.Sprintf("%s %s de cada mes", ordinal, weekdays[r.Weekday])
	default:
		return ""
	}
}
