package foundation

import (
	"testing"
	"time"
)

func TestTaiwanTradingCalendarWeekendHolidayAndCutoff(t *testing.T) {
	calendar := TaiwanTradingCalendar{Holidays: map[string]bool{"2026-09-28": true}}
	zone := time.FixedZone("Asia/Taipei", 8*60*60)
	tests := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 9, 29, 17, 31, 0, 0, zone), "2026-09-29"},
		{time.Date(2026, 9, 29, 17, 29, 0, 0, zone), "2026-09-25"},
		{time.Date(2026, 9, 27, 20, 0, 0, 0, zone), "2026-09-25"},
	}
	for _, test := range tests {
		if got := calendar.LatestCompleted(test.now, 17, 30).Format("2006-01-02"); got != test.want {
			t.Fatalf("latest completed=%s want=%s", got, test.want)
		}
	}
}
