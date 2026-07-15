package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Date is a calendar date without time or timezone.
type Date struct {
	year  int
	month time.Month
	day   int
}

func NewDate(year int, month time.Month, day int) (Date, error) {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || t.Month() != month || t.Day() != day {
		return Date{}, fmt.Errorf("invalid date")
	}
	return Date{year, month, day}, nil
}
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return Date{t.Year(), t.Month(), t.Day()}, nil
}
func DateFromTime(t time.Time) Date { y, m, d := t.Date(); return Date{y, m, d} }
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}
func (d Date) IsZero() bool                 { return d.year == 0 }
func (d Date) Time() time.Time              { return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC) }
func (d Date) Before(o Date) bool           { return d.Time().Before(o.Time()) }
func (d Date) After(o Date) bool            { return d.Time().After(o.Time()) }
func (d Date) Equal(o Date) bool            { return d == o }
func (d Date) AddDays(n int) Date           { return DateFromTime(d.Time().AddDate(0, 0, n)) }
func (d Date) AddMonths(n int) Date         { return DateFromTime(d.Time().AddDate(0, n, 0)) }
func (d Date) Weekday() time.Weekday        { return d.Time().Weekday() }
func (d Date) DaysInMonth() int             { return time.Date(d.year, d.month+1, 0, 0, 0, 0, 0, time.UTC).Day() }
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseDate(s)
	if err == nil {
		*d = v
	}
	return err
}
