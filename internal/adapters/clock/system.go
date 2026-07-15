package clock

import (
	"github.com/Polo123456789/tasks/internal/domain"
	"time"
)

type System struct{}

func (System) Now() time.Time     { return time.Now() }
func (System) Today() domain.Date { return domain.DateFromTime(time.Now()) }
