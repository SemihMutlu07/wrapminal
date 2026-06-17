package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PastedContent struct {
	Content string `json:"content"`
}

type ClaudeEntry struct {
	Display        string                   `json:"display"`
	PastedContents map[string]PastedContent `json:"pastedContents"`
	Timestamp      int64                    `json:"timestamp"`
	Project        string                   `json:"project"`
	SessionID      string                   `json:"sessionId"`
}

func (e *ClaudeEntry) EstimateChars() int {
	n := len(e.Display)
	for _, p := range e.PastedContents {
		n += len(p.Content)
	}
	return n
}

type CodexEntry struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"ts"`
	SessionID string `json:"session_id"`
}

type GeminiEntry struct {
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
	Workspace string `json:"workspace"`
}

type Interaction struct {
	Source    string
	SourceID  string
	Project   string
	SessionID string
	Timestamp time.Time
	Chars     int
}

type SourceStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	State      string `json:"state"`
	Records    int    `json:"records"`
	Confidence string `json:"confidence"`
	Message    string `json:"message"`
}

type Totals struct {
	Prompts         int `json:"prompts"`
	Sessions        int `json:"sessions"`
	Projects        int `json:"projects"`
	Sources         int `json:"sources"`
	ActiveDays      int `json:"active_days"`
	EstimatedTokens int `json:"estimated_tokens"`
}

type ProjectStats struct {
	Name       string  `json:"name"`
	Source     string  `json:"source"`
	SourceID   string  `json:"source_id"`
	Prompts    int     `json:"prompts"`
	Sessions   int     `json:"sessions"`
	First      string  `json:"first_date"`
	Last       string  `json:"last_date"`
	ActiveDays int     `json:"active_days"`
	Intensity  float64 `json:"intensity"`
	Tokens     int     `json:"tokens"`
	Confidence string  `json:"confidence"`
}

type SourceBreakdown struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Prompts  int    `json:"prompts"`
	Sessions int    `json:"sessions"`
	Tokens   int    `json:"tokens"`
}

type Highlight struct {
	Title  string `json:"title"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type WeekStats struct {
	Week        string         `json:"week"`
	Label       string         `json:"label"`
	Prompts     int            `json:"prompts"`
	Tokens      int            `json:"tokens"`
	Sessions    int            `json:"sessions"`
	Projects    map[string]int `json:"-"`
	TopProjects []ProjectCount `json:"top_projects"`
}

type ProjectCount struct {
	Name    string `json:"name"`
	Prompts int    `json:"prompts"`
}

type MonthStats struct {
	Month   string `json:"month"`
	Label   string `json:"label"`
	Prompts int    `json:"prompts"`
	Tokens  int    `json:"tokens"`
	Days    int    `json:"active_days"`
}

type Timeline struct {
	TotalTokens   int          `json:"total_tokens"`
	TotalWeeks    int          `json:"total_weeks"`
	ActiveWeeks   int          `json:"active_weeks"`
	AvgPerWeek    float64      `json:"avg_prompts_per_week"`
	AvgTokensWeek float64      `json:"avg_tokens_per_week"`
	Weeks         []WeekStats  `json:"weeks"`
	Months        []MonthStats `json:"months"`
}

type WrappedResponse struct {
	GeneratedAt     string            `json:"generated_at"`
	Sources         []SourceStatus    `json:"sources"`
	Totals          Totals            `json:"totals"`
	Projects        []ProjectStats    `json:"projects"`
	Timeline        Timeline          `json:"timeline"`
	Highlights      []Highlight       `json:"highlights"`
	SourceBreakdown []SourceBreakdown `json:"source_breakdown"`
}

type projectCollector struct {
	name     string
	source   string
	sourceID string
	prompts  int
	tokens   int
	first    time.Time
	last     time.Time
	sessions map[string]struct{}
	days     map[string]struct{}
}

func BuildWrapped() (*WrappedResponse, error) {
	home, err := ccLensHome()
	if err != nil {
		return nil, err
	}

	var interactions []Interaction
	var sources []SourceStatus

	for _, parser := range []func(string) ([]Interaction, SourceStatus){
		parseClaude,
		parseCodex,
		parseGemini,
		detectOpenCode,
		detectCursor,
	} {
		items, status := parser(home)
		sources = append(sources, status)
		interactions = append(interactions, items...)
	}

	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.Before(interactions[j].Timestamp)
	})

	totals, projects, timeline, sourceBreakdown := aggregate(interactions)

	return &WrappedResponse{
		GeneratedAt:     time.Now().Format(time.RFC3339),
		Sources:         sources,
		Totals:          totals,
		Projects:        projects,
		Timeline:        timeline,
		Highlights:      buildHighlights(interactions, projects, timeline, sourceBreakdown),
		SourceBreakdown: sourceBreakdown,
	}, nil
}

func ParseHistory() ([]ProjectStats, error) {
	wrapped, err := BuildWrapped()
	if err != nil {
		return nil, err
	}
	return wrapped.Projects, nil
}

func ParseTimeline() (*Timeline, error) {
	wrapped, err := BuildWrapped()
	if err != nil {
		return nil, err
	}
	return &wrapped.Timeline, nil
}

func ccLensHome() (string, error) {
	if override := os.Getenv("CC_LENS_HOME"); override != "" {
		return override, nil
	}
	return os.UserHomeDir()
}

func parseClaude(home string) ([]Interaction, SourceStatus) {
	path := filepath.Join(home, ".claude", "history.jsonl")
	status := SourceStatus{
		ID:         "claude",
		Name:       "Claude Code",
		Path:       path,
		State:      "missing",
		Confidence: "estimated",
		Message:    "No Claude Code history found.",
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status
		}
		status.State = "error"
		status.Message = err.Error()
		return nil, status
	}
	defer file.Close()

	var items []Interaction
	scanner := newJSONLScanner(file)
	for scanner.Scan() {
		var entry ClaudeEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Timestamp == 0 {
			continue
		}

		items = append(items, Interaction{
			Source:    status.Name,
			SourceID:  status.ID,
			Project:   projectName(entry.Project, "Unknown Claude Project"),
			SessionID: fallbackSession(entry.SessionID, status.ID, len(items)),
			Timestamp: unixFlexible(entry.Timestamp),
			Chars:     entry.EstimateChars(),
		})
	}

	if err := scanner.Err(); err != nil {
		status.State = "error"
		status.Message = err.Error()
		return items, status
	}

	status.Records = len(items)
	status.State = loadedState(items)
	status.Message = loadedMessage(items, "Claude prompts loaded.")
	return items, status
}

func parseCodex(home string) ([]Interaction, SourceStatus) {
	path := filepath.Join(home, ".codex", "history.jsonl")
	status := SourceStatus{
		ID:         "codex",
		Name:       "Codex CLI",
		Path:       path,
		State:      "missing",
		Confidence: "estimated",
		Message:    "No Codex CLI history found.",
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status
		}
		status.State = "error"
		status.Message = err.Error()
		return nil, status
	}
	defer file.Close()

	var items []Interaction
	scanner := newJSONLScanner(file)
	for scanner.Scan() {
		var entry CodexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Timestamp == 0 {
			continue
		}

		items = append(items, Interaction{
			Source:    status.Name,
			SourceID:  status.ID,
			Project:   "Codex CLI",
			SessionID: fallbackSession(entry.SessionID, status.ID, len(items)),
			Timestamp: unixFlexible(entry.Timestamp),
			Chars:     len(entry.Text),
		})
	}

	if err := scanner.Err(); err != nil {
		status.State = "error"
		status.Message = err.Error()
		return items, status
	}

	status.Records = len(items)
	status.State = loadedState(items)
	status.Message = loadedMessage(items, "Codex prompts loaded. Project names are not exposed by history.jsonl, so Codex is grouped as one source.")
	return items, status
}

func parseGemini(home string) ([]Interaction, SourceStatus) {
	paths := []string{
		filepath.Join(home, ".gemini", "antigravity-cli", "history.jsonl"),
		filepath.Join(home, ".gemini", "history.jsonl"),
	}
	status := SourceStatus{
		ID:         "gemini",
		Name:       "Gemini / Antigravity",
		Path:       strings.Join(paths, ", "),
		State:      "missing",
		Confidence: "estimated",
		Message:    "No Gemini CLI history found.",
	}

	var items []Interaction
	var existing []string
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		existing = append(existing, path)

		scanner := newJSONLScanner(file)
		for scanner.Scan() {
			var entry GeminiEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Timestamp == 0 {
				continue
			}

			items = append(items, Interaction{
				Source:    status.Name,
				SourceID:  status.ID,
				Project:   projectName(entry.Workspace, "Gemini Workspace"),
				SessionID: fallbackSession("", status.ID, len(items)),
				Timestamp: unixFlexible(entry.Timestamp),
				Chars:     len(entry.Display),
			})
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			status.State = "error"
			status.Message = err.Error()
			return items, status
		}
		_ = file.Close()
	}

	if len(existing) > 0 {
		status.Path = strings.Join(existing, ", ")
		status.Records = len(items)
		status.State = loadedState(items)
		status.Message = loadedMessage(items, "Gemini/Antigravity prompts loaded.")
	}
	return items, status
}

func detectOpenCode(home string) ([]Interaction, SourceStatus) {
	paths := []string{
		filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
		filepath.Join(home, "Library", "Application Support", "opencode", "opencode.db"),
		filepath.Join(home, "AppData", "Roaming", "opencode", "opencode.db"),
	}
	status := SourceStatus{
		ID:         "opencode",
		Name:       "OpenCode",
		Path:       strings.Join(paths, ", "),
		State:      "missing",
		Confidence: "detected",
		Message:    "No OpenCode database found.",
	}

	for _, path := range paths {
		if fileExists(path) {
			status.Path = path
			status.State = "detected"
			status.Message = "OpenCode SQLite history detected. Parser is marked experimental until the storage format is stabilized or exported."
			return nil, status
		}
	}
	return nil, status
}

func detectCursor(home string) ([]Interaction, SourceStatus) {
	paths := []string{
		filepath.Join(home, ".config", "Cursor"),
		filepath.Join(home, "Library", "Application Support", "Cursor"),
		filepath.Join(home, "AppData", "Roaming", "Cursor"),
	}
	status := SourceStatus{
		ID:         "cursor",
		Name:       "Cursor",
		Path:       strings.Join(paths, ", "),
		State:      "missing",
		Confidence: "detected",
		Message:    "No Cursor local storage found.",
	}

	for _, path := range paths {
		if fileExists(path) {
			status.Path = path
			status.State = "detected"
			status.Message = "Cursor storage detected. Local chat history is not treated as a stable public format yet."
			return nil, status
		}
	}
	return nil, status
}

func aggregate(items []Interaction) (Totals, []ProjectStats, Timeline, []SourceBreakdown) {
	totals := Totals{Prompts: len(items)}
	projectMap := make(map[string]*projectCollector)
	sourceMap := make(map[string]*SourceBreakdown)
	sourceSessions := make(map[string]map[string]struct{})
	allSessions := make(map[string]struct{})
	allDays := make(map[string]struct{})

	for _, item := range items {
		if item.Timestamp.IsZero() {
			continue
		}
		tokens := estimateTokens(item.Chars)
		totals.EstimatedTokens += tokens
		allSessions[item.SourceID+":"+item.SessionID] = struct{}{}
		allDays[item.Timestamp.Format("2006-01-02")] = struct{}{}

		key := item.SourceID + ":" + item.Project
		c, ok := projectMap[key]
		if !ok {
			c = &projectCollector{
				name:     item.Project,
				source:   item.Source,
				sourceID: item.SourceID,
				first:    item.Timestamp,
				last:     item.Timestamp,
				sessions: make(map[string]struct{}),
				days:     make(map[string]struct{}),
			}
			projectMap[key] = c
		}
		c.prompts++
		c.tokens += tokens
		c.sessions[item.SessionID] = struct{}{}
		c.days[item.Timestamp.Format("2006-01-02")] = struct{}{}
		if item.Timestamp.Before(c.first) {
			c.first = item.Timestamp
		}
		if item.Timestamp.After(c.last) {
			c.last = item.Timestamp
		}

		sb, ok := sourceMap[item.SourceID]
		if !ok {
			sb = &SourceBreakdown{ID: item.SourceID, Name: item.Source}
			sourceMap[item.SourceID] = sb
		}
		sb.Prompts++
		sb.Tokens += tokens

		if _, ok := sourceSessions[item.SourceID]; !ok {
			sourceSessions[item.SourceID] = make(map[string]struct{})
		}
		sourceSessions[item.SourceID][item.SessionID] = struct{}{}
	}

	totals.Sessions = len(allSessions)
	totals.ActiveDays = len(allDays)
	totals.Projects = len(projectMap)
	totals.Sources = len(sourceMap)

	projects := make([]ProjectStats, 0, len(projectMap))
	for _, c := range projectMap {
		totalDays := int(c.last.Sub(c.first).Hours()/24) + 1
		if totalDays < 1 {
			totalDays = 1
		}
		projects = append(projects, ProjectStats{
			Name:       c.name,
			Source:     c.source,
			SourceID:   c.sourceID,
			Prompts:    c.prompts,
			Sessions:   len(c.sessions),
			First:      c.first.Format("2006-01-02"),
			Last:       c.last.Format("2006-01-02"),
			ActiveDays: len(c.days),
			Intensity:  round1(float64(c.prompts) / float64(totalDays)),
			Tokens:     c.tokens,
			Confidence: "estimated",
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Prompts == projects[j].Prompts {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].Prompts > projects[j].Prompts
	})

	breakdown := make([]SourceBreakdown, 0, len(sourceMap))
	for _, sb := range sourceMap {
		sb.Sessions = len(sourceSessions[sb.ID])
		breakdown = append(breakdown, *sb)
	}
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].Prompts > breakdown[j].Prompts })

	return totals, projects, buildTimeline(items), breakdown
}

func buildTimeline(items []Interaction) Timeline {
	weeks := make(map[string]*WeekStats)
	months := make(map[string]*MonthStats)
	monthDays := make(map[string]map[string]struct{})
	allSessions := make(map[string]map[string]struct{})
	totalTokens := 0
	var earliest, latest time.Time

	for _, item := range items {
		if item.Timestamp.IsZero() {
			continue
		}
		tokens := estimateTokens(item.Chars)
		totalTokens += tokens

		if earliest.IsZero() || item.Timestamp.Before(earliest) {
			earliest = item.Timestamp
		}
		if latest.IsZero() || item.Timestamp.After(latest) {
			latest = item.Timestamp
		}

		y, w := item.Timestamp.ISOWeek()
		weekStart := startOfISOWeek(y, w, item.Timestamp.Location())
		weekEnd := weekStart.AddDate(0, 0, 6)
		weekKey := fmt.Sprintf("%04d-W%02d", y, w)

		ws, ok := weeks[weekKey]
		if !ok {
			ws = &WeekStats{
				Week:     weekKey,
				Label:    weekStart.Format("Jan 02") + " - " + weekEnd.Format("Jan 02"),
				Projects: make(map[string]int),
			}
			weeks[weekKey] = ws
		}
		ws.Prompts++
		ws.Tokens += tokens
		ws.Projects[item.Project]++

		if _, ok := allSessions[weekKey]; !ok {
			allSessions[weekKey] = make(map[string]struct{})
		}
		allSessions[weekKey][item.SourceID+":"+item.SessionID] = struct{}{}

		monthKey := item.Timestamp.Format("2006-01")
		ms, ok := months[monthKey]
		if !ok {
			ms = &MonthStats{Month: monthKey, Label: item.Timestamp.Format("Jan 2006")}
			months[monthKey] = ms
		}
		ms.Prompts++
		ms.Tokens += tokens

		if _, ok := monthDays[monthKey]; !ok {
			monthDays[monthKey] = make(map[string]struct{})
		}
		monthDays[monthKey][item.Timestamp.Format("2006-01-02")] = struct{}{}
	}

	weekList := make([]WeekStats, 0, len(weeks))
	for key, ws := range weeks {
		ws.Sessions = len(allSessions[key])
		counts := make([]ProjectCount, 0, len(ws.Projects))
		for name, count := range ws.Projects {
			counts = append(counts, ProjectCount{Name: name, Prompts: count})
		}
		sort.Slice(counts, func(i, j int) bool { return counts[i].Prompts > counts[j].Prompts })
		if len(counts) > 3 {
			counts = counts[:3]
		}
		ws.TopProjects = counts
		weekList = append(weekList, *ws)
	}
	sort.Slice(weekList, func(i, j int) bool { return weekList[i].Week < weekList[j].Week })

	monthList := make([]MonthStats, 0, len(months))
	for key, ms := range months {
		ms.Days = len(monthDays[key])
		monthList = append(monthList, *ms)
	}
	sort.Slice(monthList, func(i, j int) bool { return monthList[i].Month < monthList[j].Month })

	totalWeeks := 0
	if !earliest.IsZero() && !latest.IsZero() {
		totalWeeks = int(latest.Sub(earliest).Hours()/(24*7)) + 1
		if totalWeeks < 1 {
			totalWeeks = 1
		}
	}

	return Timeline{
		TotalTokens:   totalTokens,
		TotalWeeks:    totalWeeks,
		ActiveWeeks:   len(weekList),
		AvgPerWeek:    avgPromptsPerWeek(weekList),
		AvgTokensWeek: avgTokensPerWeek(weekList),
		Weeks:         weekList,
		Months:        monthList,
	}
}

func buildHighlights(items []Interaction, projects []ProjectStats, timeline Timeline, sources []SourceBreakdown) []Highlight {
	if len(items) == 0 {
		return []Highlight{
			{Title: "No local history", Value: "0", Detail: "Use sample mode to preview the wrapped layout."},
		}
	}

	var highlights []Highlight
	if len(projects) > 0 {
		top := projects[0]
		highlights = append(highlights, Highlight{
			Title:  "Top project",
			Value:  top.Name,
			Detail: fmt.Sprintf("%d prompts via %s", top.Prompts, top.Source),
		})
	}

	if len(sources) > 0 {
		top := sources[0]
		highlights = append(highlights, Highlight{
			Title:  "Main agent",
			Value:  top.Name,
			Detail: fmt.Sprintf("%d prompts captured locally", top.Prompts),
		})
	}

	if week := busiestWeek(timeline.Weeks); week != nil {
		highlights = append(highlights, Highlight{
			Title:  "Busiest week",
			Value:  week.Label,
			Detail: fmt.Sprintf("%d prompts across %d sessions", week.Prompts, week.Sessions),
		})
	}

	streak := longestStreak(items)
	highlights = append(highlights, Highlight{
		Title:  "Longest streak",
		Value:  fmt.Sprintf("%d days", streak),
		Detail: "Consecutive days with AI coding activity",
	})

	hour, count := peakHour(items)
	highlights = append(highlights, Highlight{
		Title:  "Peak hour",
		Value:  fmt.Sprintf("%02d:00", hour),
		Detail: fmt.Sprintf("%d prompts started in this hour", count),
	})

	weekend, pct := weekendShare(items)
	highlights = append(highlights, Highlight{
		Title:  "Weekend mode",
		Value:  fmt.Sprintf("%d prompts", weekend),
		Detail: fmt.Sprintf("%.0f%% of all captured activity", pct),
	})

	return highlights
}

func newJSONLScanner(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	return scanner
}

func projectName(path string, fallback string) string {
	if strings.TrimSpace(path) == "" {
		return fallback
	}
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fallback
	}
	return name
}

func fallbackSession(id string, source string, index int) string {
	if id != "" {
		return id
	}
	return fmt.Sprintf("%s-session-%d", source, index)
}

func unixFlexible(ts int64) time.Time {
	switch {
	case ts > 1_000_000_000_000:
		return time.UnixMilli(ts)
	case ts > 0:
		return time.Unix(ts, 0)
	default:
		return time.Time{}
	}
}

func estimateTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	return int(math.Ceil(float64(chars) / 4.0))
}

func loadedState(items []Interaction) string {
	if len(items) == 0 {
		return "detected"
	}
	return "loaded"
}

func loadedMessage(items []Interaction, message string) string {
	if len(items) == 0 {
		return "History file found, but no usable records were parsed."
	}
	return message
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func round1(n float64) float64 {
	return math.Round(n*10) / 10
}

func startOfISOWeek(year int, week int, loc *time.Location) time.Time {
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, loc)
	offset := (int(jan4.Weekday()) + 6) % 7
	week1 := jan4.AddDate(0, 0, -offset)
	return week1.AddDate(0, 0, (week-1)*7)
}

func avgPromptsPerWeek(weeks []WeekStats) float64 {
	if len(weeks) == 0 {
		return 0
	}
	total := 0
	for _, week := range weeks {
		total += week.Prompts
	}
	return round1(float64(total) / float64(len(weeks)))
}

func avgTokensPerWeek(weeks []WeekStats) float64 {
	if len(weeks) == 0 {
		return 0
	}
	total := 0
	for _, week := range weeks {
		total += week.Tokens
	}
	return math.Round(float64(total) / float64(len(weeks)))
}

func busiestWeek(weeks []WeekStats) *WeekStats {
	if len(weeks) == 0 {
		return nil
	}
	top := weeks[0]
	for _, week := range weeks[1:] {
		if week.Prompts > top.Prompts {
			top = week
		}
	}
	return &top
}

func longestStreak(items []Interaction) int {
	days := make(map[string]time.Time)
	for _, item := range items {
		day := time.Date(item.Timestamp.Year(), item.Timestamp.Month(), item.Timestamp.Day(), 0, 0, 0, 0, item.Timestamp.Location())
		days[day.Format("2006-01-02")] = day
	}
	if len(days) == 0 {
		return 0
	}

	list := make([]time.Time, 0, len(days))
	for _, day := range days {
		list = append(list, day)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Before(list[j]) })

	best := 1
	current := 1
	for i := 1; i < len(list); i++ {
		if list[i].Sub(list[i-1]).Hours() == 24 {
			current++
			if current > best {
				best = current
			}
		} else {
			current = 1
		}
	}
	return best
}

func peakHour(items []Interaction) (int, int) {
	counts := make(map[int]int)
	for _, item := range items {
		counts[item.Timestamp.Hour()]++
	}
	topHour := 0
	topCount := 0
	for hour, count := range counts {
		if count > topCount || (count == topCount && hour < topHour) {
			topHour = hour
			topCount = count
		}
	}
	return topHour, topCount
}

func weekendShare(items []Interaction) (int, float64) {
	if len(items) == 0 {
		return 0, 0
	}
	weekend := 0
	for _, item := range items {
		if item.Timestamp.Weekday() == time.Saturday || item.Timestamp.Weekday() == time.Sunday {
			weekend++
		}
	}
	return weekend, float64(weekend) / float64(len(items)) * 100
}
