package supplier

import (
	"testing"
	"time"
)

func TestHighPassServiceDate(t *testing.T) {
	// A day split into two windows, with a 01:00-04:00 gap between them.
	split := []ServiceHour{{Open: "04:00", Close: "23:59"}, {Open: "00:00", Close: "01:00"}}
	// A day with a single window.
	single := []ServiceHour{{Open: "04:00", Close: "21:00"}}

	at := func(value string) time.Time {
		parsed, err := time.Parse(time.DateTime, value)
		if err != nil {
			t.Fatalf("bad fixture time %q: %v", value, err)
		}
		return parsed
	}

	tests := []struct {
		name  string
		date  string
		hours []ServiceHour
		now   time.Time
		want  string
	}{
		{
			name: "later date opening at midnight keeps the bare date",
			date: "2026-09-19", hours: split, now: at("2026-08-25 15:00:00"),
			want: "",
		},
		{
			name: "later date opening at 04:00 gets the opening plus the buffer",
			date: "2026-09-19", hours: single, now: at("2026-08-25 15:00:00"),
			want: "2026-09-19 04:10:00",
		},
		{
			name: "same day before opening waits for the opening",
			date: "2026-08-25", hours: single, now: at("2026-08-25 02:00:00"),
			want: "2026-08-25 04:10:00",
		},
		{
			name: "same day while open moves to now",
			date: "2026-08-25", hours: single, now: at("2026-08-25 10:00:00"),
			want: "2026-08-25 10:10:00",
		},
		{
			name: "same day inside the gap skips to the next window",
			date: "2026-08-25", hours: split, now: at("2026-08-25 02:30:00"),
			want: "2026-08-25 04:10:00",
		},
		{
			name: "same day inside the first window stays in it",
			date: "2026-08-25", hours: split, now: at("2026-08-25 00:30:00"),
			want: "2026-08-25 00:40:00",
		},
		{
			name: "same day after the last closing falls back to now",
			date: "2026-08-25", hours: single, now: at("2026-08-25 22:00:00"),
			want: "2026-08-25 22:10:00",
		},
		{
			name: "a window without a closing hour runs to end of day",
			date: "2026-09-19", hours: []ServiceHour{{Open: "06:30"}}, now: at("2026-08-25 15:00:00"),
			want: "2026-09-19 06:40:00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := highPassServiceDate(test.date, test.hours, test.now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestHighPassServiceDateRejectsBadInput(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)

	if _, err := highPassServiceDate("19.09.2026", []ServiceHour{{Open: "04:00"}}, now); err == nil {
		t.Error("expected an error for a malformed date")
	}
	if _, err := highPassServiceDate("2026-09-19", []ServiceHour{{Open: "4am"}}, now); err == nil {
		t.Error("expected an error for a malformed opening hour")
	}
	if _, err := HighPassServiceDate("2026-09-19", []ServiceHour{{Open: "04:00"}}, "not-an-offset"); err == nil {
		t.Error("expected an error for a malformed timezone offset")
	}
}
