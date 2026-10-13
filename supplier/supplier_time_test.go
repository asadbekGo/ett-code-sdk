package supplier

import (
	"testing"
	"time"
)

// wallClock re-reads a local time as the bare wall clock, the way time.Parse reads the
// time.DateTime strings the suppliers are sent. Comparing the two directly would otherwise be
// off by the test machine's own zone offset.
func wallClock(t *testing.T, at time.Time) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.DateTime, at.Format(time.DateTime))
	if err != nil {
		t.Fatalf("re-reading %v as a wall clock: %v", at, err)
	}

	return parsed
}

func TestParseTimezoneOffset(t *testing.T) {
	tests := []struct {
		name   string
		offset string
		want   time.Duration
	}{
		{name: "whole hours", offset: "5:00", want: 5 * time.Hour},
		{name: "utc", offset: "0:00", want: 0},
		{name: "half-hour zone", offset: "5:45", want: 5*time.Hour + 45*time.Minute},
		{name: "negative offset carries the sign into the minutes", offset: "-3:30", want: -(3*time.Hour + 30*time.Minute)},
		{name: "two digit hours", offset: "+11:00", want: 11 * time.Hour},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTimezoneOffset(test.offset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestParseTimezoneOffsetRejectsBadInput(t *testing.T) {
	for _, offset := range []string{"", "5", "not-an-offset", "5:xx", "h:m"} {
		if _, err := parseTimezoneOffset(offset); err == nil {
			t.Errorf("expected an error for offset %q", offset)
		}
	}
}

func TestDayTime(t *testing.T) {
	date := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		clock string
		want  time.Time
	}{
		{clock: "00:00", want: date},
		{clock: "04:00", want: date.Add(4 * time.Hour)},
		{clock: "23:59", want: date.Add(23*time.Hour + 59*time.Minute)},
		{clock: "06:30", want: date.Add(6*time.Hour + 30*time.Minute)},
	}

	for _, test := range tests {
		t.Run(test.clock, func(t *testing.T) {
			got, err := dayTime(date, test.clock)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}

	for _, clock := range []string{"", "4am", "24:00", "4:00pm"} {
		if _, err := dayTime(date, clock); err == nil {
			t.Errorf("expected an error for clock %q", clock)
		}
	}
}

func TestHighPassServiceDateAppliesTheTimezoneOffset(t *testing.T) {
	// A window covering the whole day means the answer is driven purely by "now" in the
	// product's local zone, which is what the offset is there to produce.
	wholeDay := []ServiceHour{{Open: "00:00", Close: "23:59"}}
	nowLocal := time.Now().UTC().Add(7 * time.Hour)
	today := nowLocal.Format(time.DateOnly)

	got, err := HighPassServiceDate(today, wholeDay, "7:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serviceDate, err := time.Parse(time.DateTime, got)
	if err != nil {
		t.Fatalf("service date %q is not in the supplier's format: %v", got, err)
	}
	if drift := serviceDate.Sub(nowLocal.Add(serviceDateBuffer)); drift > time.Minute || drift < -time.Minute {
		t.Errorf("got %v, want local now plus the %v buffer (off by %v)", serviceDate, serviceDateBuffer, drift)
	}
}

func TestHighPassServiceDateKeepsTheEarliestOpenWindow(t *testing.T) {
	// Windows arrive in no particular order; the earliest one that can still be served wins.
	unordered := []ServiceHour{
		{Open: "18:00", Close: "23:00"},
		{Open: "06:00", Close: "12:00"},
		{Open: "13:00", Close: "17:00"},
	}
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)

	got, err := highPassServiceDate("2026-09-19", unordered, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "2026-09-19 06:10:00"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHighPassServiceDateWithNoWindows(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)

	t.Run("a future date keeps the bare date", func(t *testing.T) {
		got, err := highPassServiceDate("2026-09-19", nil, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want the bare date kept", got)
		}
	})

	t.Run("today falls back to now plus the buffer", func(t *testing.T) {
		got, err := highPassServiceDate("2026-08-25", nil, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "2026-08-25 15:10:00"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestHighPassServiceDateDoesNotMutateTheCallersWindows(t *testing.T) {
	// The function sorts the windows, so it must sort a copy — the caller reuses the slice
	// for every passenger on the same product.
	windows := []ServiceHour{{Open: "18:00", Close: "23:00"}, {Open: "06:00", Close: "12:00"}}
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)

	if _, err := highPassServiceDate("2026-09-19", windows, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if windows[0].Open != "18:00" || windows[1].Open != "06:00" {
		t.Errorf("got %v, want the caller's order untouched", windows)
	}
}

func TestHighPassServiceDateRejectsABadClosingHour(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC)

	if _, err := highPassServiceDate("2026-09-19", []ServiceHour{{Open: "04:00", Close: "9pm"}}, now); err == nil {
		t.Error("expected an error for a malformed closing hour")
	}
}

func TestIsCurrentDate(t *testing.T) {
	t.Run("today in the product's zone", func(t *testing.T) {
		if !IsCurrentDate(time.Now().Format(time.DateOnly), "0:00") {
			t.Error("today at a zero offset should be the current date")
		}
	})

	t.Run("the offset moves which date counts as today", func(t *testing.T) {
		// A 48-hour offset puts "today" two days out, which is the whole point of the offset.
		twoDaysOut := time.Now().Add(48 * time.Hour).Format(time.DateOnly)
		if !IsCurrentDate(twoDaysOut, "48:00") {
			t.Errorf("%s should be the current date at a 48-hour offset", twoDaysOut)
		}
		if IsCurrentDate(twoDaysOut, "0:00") {
			t.Errorf("%s should not be the current date at a zero offset", twoDaysOut)
		}
	})

	t.Run("a later date is not today", func(t *testing.T) {
		if IsCurrentDate(time.Now().AddDate(0, 0, 3).Format(time.DateOnly), "0:00") {
			t.Error("a date three days out should not be the current date")
		}
	})

	t.Run("bad input is reported as not today", func(t *testing.T) {
		if IsCurrentDate("18.09.2026", "0:00") {
			t.Error("a malformed date should not be the current date")
		}
		if IsCurrentDate(time.Now().Format(time.DateOnly), "x:y") {
			t.Error("a malformed offset should not be the current date")
		}
	})
}

func TestAdd10Minutes(t *testing.T) {
	t.Run("returns local now plus ten minutes", func(t *testing.T) {
		before := time.Now().Add(5 * time.Hour).Add(10 * time.Minute)

		got, err := Add10Minutes("5:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		parsed, err := time.Parse(time.DateTime, got)
		if err != nil {
			t.Fatalf("result %q is not in the supplier's format: %v", got, err)
		}
		wantWallClock := wallClock(t, before)
		if drift := parsed.Sub(wantWallClock); drift > time.Minute || drift < -time.Minute {
			t.Errorf("got %q, want within a minute of %v (off by %v)", got, wantWallClock, drift)
		}
	})

	t.Run("rejects a malformed offset", func(t *testing.T) {
		if _, err := Add10Minutes("x:y"); err == nil {
			t.Error("expected an error for a malformed offset")
		}
	})
}

// TestOffsetWithoutAColonPanics pins down a real defect rather than endorsing it: unlike
// parseTimezoneOffset, which returns an error, IsCurrentDate and Add10Minutes index into
// strings.Split(offset, ":") without checking its length, so any offset lacking a colon —
// an empty one from missing product data included — takes the process down. CreateOrder
// reaches both with product data straight from the database.
func TestOffsetWithoutAColonPanics(t *testing.T) {
	for _, offset := range []string{"", "5"} {
		t.Run("IsCurrentDate/"+offset, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("IsCurrentDate no longer panics on a colonless offset — fix the test, the defect is gone")
				}
			}()
			IsCurrentDate(time.Now().Format(time.DateOnly), offset)
		})

		t.Run("Add10Minutes/"+offset, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Add10Minutes no longer panics on a colonless offset — fix the test, the defect is gone")
				}
			}()
			Add10Minutes(offset)
		})
	}
}
