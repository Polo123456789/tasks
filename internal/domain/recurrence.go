package domain

import (
	"fmt"
	"sort"
	"time"
)

type RecurrenceKind string

const (
	Daily          RecurrenceKind = "daily"
	Weekly         RecurrenceKind = "weekly"
	MonthlyDay     RecurrenceKind = "monthly_day"
	MonthlyLastDay RecurrenceKind = "monthly_last_day"
	MonthlyWeekday RecurrenceKind = "monthly_weekday"
)

type Recurrence struct {
	Kind     RecurrenceKind `json:"kind"`
	Weekdays []time.Weekday `json:"weekdays,omitempty"`
	Day      int            `json:"day,omitempty"`
	Ordinal  int            `json:"ordinal,omitempty"`
	Weekday  time.Weekday   `json:"weekday,omitempty"`
}

func (r Recurrence) Validate() error {
	switch r.Kind {
	case Daily, MonthlyLastDay:
		return nil
	case Weekly:
		if len(r.Weekdays) == 0 {
			return fmt.Errorf("weekly recurrence requires weekdays")
		}
		for _, d := range r.Weekdays {
			if d < 0 || d > 6 {
				return fmt.Errorf("invalid weekday")
			}
		}
	case MonthlyDay:
		if r.Day < 1 || r.Day > 31 {
			return fmt.Errorf("invalid month day")
		}
	case MonthlyWeekday:
		if (r.Ordinal < 1 || r.Ordinal > 4) && r.Ordinal != -1 {
			return fmt.Errorf("invalid ordinal")
		}
		if r.Weekday < 0 || r.Weekday > 6 {
			return fmt.Errorf("invalid weekday")
		}
	default:
		return fmt.Errorf("invalid recurrence kind")
	}
	return nil
}
func (r Recurrence) Next(after Date) (Date, error) {
	if err := r.Validate(); err != nil {
		return Date{}, err
	}
	switch r.Kind {
	case Daily:
		return after.AddDays(1), nil
	case Weekly:
		days := append([]time.Weekday(nil), r.Weekdays...)
		sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
		for n := 1; n <= 7; n++ {
			d := after.AddDays(n)
			for _, w := range days {
				if d.Weekday() == w {
					return d, nil
				}
			}
		}
	case MonthlyDay:
		for n := 1; n <= 24; n++ {
			base := DateFromTime(time.Date(after.year, after.month+n, 1, 0, 0, 0, 0, time.UTC))
			day := r.Day
			if day > base.DaysInMonth() {
				continue
			}
			d, _ := NewDate(base.year, base.month, day)
			if d.After(after) {
				return d, nil
			}
		}
	case MonthlyLastDay:
		base := DateFromTime(time.Date(after.year, after.month+1, 1, 0, 0, 0, 0, time.UTC))
		return NewDate(base.year, base.month, base.DaysInMonth())
	case MonthlyWeekday:
		for n := 1; n <= 24; n++ {
			base := DateFromTime(time.Date(after.year, after.month+n, 1, 0, 0, 0, 0, time.UTC))
			d := ordinalWeekday(base.year, base.month, r.Ordinal, r.Weekday)
			if d.After(after) {
				return d, nil
			}
		}
	}
	return Date{}, fmt.Errorf("cannot calculate next recurrence")
}
func ordinalWeekday(y int, m time.Month, ordinal int, w time.Weekday) Date {
	if ordinal == -1 {
		last, _ := NewDate(y, m, time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day())
		return last.AddDays(-((int(last.Weekday()) - int(w) + 7) % 7))
	}
	first, _ := NewDate(y, m, 1)
	return first.AddDays((int(w)-int(first.Weekday())+7)%7 + (ordinal-1)*7)
}
