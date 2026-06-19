package main

import (
	"testing"
	"time"
)

func mkItem(srcID, project, session string, t time.Time, chars int) Interaction {
	return Interaction{Source: srcID, SourceID: srcID, Project: project, SessionID: session, Timestamp: t, Chars: chars}
}

func day(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

func TestStatsMath_LongestStreak(t *testing.T) {
	if got := longestStreak(nil); got != 0 {
		t.Fatalf("empty: expected 0, got %d", got)
	}

	single := []Interaction{mkItem("s1", "p", "sess", day(2026, time.January, 1, 10), 10)}
	if got := longestStreak(single); got != 1 {
		t.Fatalf("single day: expected 1, got %d", got)
	}

	threeConsecutive := []Interaction{
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 10), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 2, 10), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 3, 10), 10),
	}
	if got := longestStreak(threeConsecutive); got != 3 {
		t.Fatalf("three consecutive days: expected 3, got %d", got)
	}

	withGap := []Interaction{
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 10), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 2, 10), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 5, 10), 10),
	}
	if got := longestStreak(withGap); got != 2 {
		t.Fatalf("gap: expected 2, got %d", got)
	}

	duplicatesSameDay := []Interaction{
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 8), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 20), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 23), 10),
	}
	if got := longestStreak(duplicatesSameDay); got != 1 {
		t.Fatalf("duplicates same day: expected 1, got %d", got)
	}
}

func TestStatsMath_PeakHour(t *testing.T) {
	hour, count := peakHour(nil)
	if hour != 0 || count != 0 {
		t.Fatalf("empty: expected (0,0), got (%d,%d)", hour, count)
	}

	clearWinner := []Interaction{
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 14), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 2, 14), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 3, 9), 10),
	}
	hour, count = peakHour(clearWinner)
	if hour != 14 || count != 2 {
		t.Fatalf("clear winner: expected (14,2), got (%d,%d)", hour, count)
	}

	tie := []Interaction{
		mkItem("s1", "p", "sess", day(2026, time.January, 1, 9), 10),
		mkItem("s1", "p", "sess", day(2026, time.January, 2, 20), 10),
	}
	hour, count = peakHour(tie)
	if hour != 9 || count != 1 {
		t.Fatalf("tie: expected smallest hour (9,1), got (%d,%d)", hour, count)
	}
}

func TestStatsMath_WeekendShare(t *testing.T) {
	count, pct := weekendShare(nil)
	if count != 0 || pct != 0.0 {
		t.Fatalf("empty: expected (0,0.0), got (%d,%f)", count, pct)
	}

	// 2026-06-20 is Saturday, 2026-06-19 is Friday.
	mixed := []Interaction{
		mkItem("s1", "p", "sess", time.Date(2026, time.June, 19, 10, 0, 0, 0, time.UTC), 10),
		mkItem("s1", "p", "sess", time.Date(2026, time.June, 19, 11, 0, 0, 0, time.UTC), 10),
		mkItem("s1", "p", "sess", time.Date(2026, time.June, 20, 10, 0, 0, 0, time.UTC), 10),
	}
	count, pct = weekendShare(mixed)
	wantPct := float64(1) / float64(3) * 100
	if count != 1 || pct != wantPct {
		t.Fatalf("mixed: expected (1,%f), got (%d,%f)", wantPct, count, pct)
	}
}

func TestStatsMath_EstimateTokens(t *testing.T) {
	cases := []struct {
		chars int
		want  int
	}{
		{0, 0},
		{1, 1},
		{4, 1},
		{5, 2},
	}
	for _, c := range cases {
		if got := estimateTokens(c.chars); got != c.want {
			t.Fatalf("estimateTokens(%d): expected %d, got %d", c.chars, c.want, got)
		}
	}
}

func TestStatsMath_SessionsBaseline(t *testing.T) {
	items := []Interaction{
		mkItem("gemini", "p", "gemini-session-0", day(2026, time.January, 1, 10), 10),
		mkItem("gemini", "p", "gemini-session-1", day(2026, time.January, 1, 11), 10),
		mkItem("gemini", "p", "gemini-session-2", day(2026, time.January, 1, 12), 10),
	}

	totals, _, _, _ := aggregate(items)

	// BASELINE: synthetic per-item session IDs inflate the count. Plan 004 changes this; update the expected value when 004 lands.
	if totals.Sessions != 3 {
		t.Fatalf("baseline session inflation: expected 3, got %d", totals.Sessions)
	}
}
