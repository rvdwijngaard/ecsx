package logs

import "testing"

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"", LevelAll},
		{"INF", LevelINF},
		{"info", LevelINF},
		{"WARN", LevelWARN},
		{"warning", LevelWARN},
		{"ERR", LevelERR},
		{"error", LevelERR},
		{"INF|WARN", LevelINF | LevelWARN},
		{"INF,WARN,ERR", LevelINF | LevelWARN | LevelERR},
		{"err|inf", LevelINF | LevelERR},
		{"INFO|WARNING|ERROR", LevelINF | LevelWARN | LevelERR},
		{"bogus", LevelAll},
		{"INF|bogus", LevelINF},
	}
	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.want {
			t.Errorf("ParseLevel(%q) = %d (%s), want %d (%s)", tt.input, got, got, tt.want, tt.want)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelAll, "ALL"},
		{LevelINF, "INF"},
		{LevelWARN, "WARN"},
		{LevelERR, "ERR"},
		{LevelINF | LevelWARN, "INF|WARN"},
		{LevelINF | LevelWARN | LevelERR, "INF|WARN|ERR"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevelMatches(t *testing.T) {
	tests := []struct {
		level Level
		msg   string
		want  bool
	}{
		{LevelAll, "anything at all", true},
		{LevelAll, "", true},
		{LevelINF, "2026-04-14 12:11:40 INF starting up", true},
		{LevelINF, "2026-04-14 12:11:40 INFO starting up", true},
		{LevelINF, "2026-04-14 12:11:40 DBG executing cycle", false},
		{LevelWARN, "WARN low memory", true},
		{LevelWARN, "WARNING: disk full", true},
		{LevelWARN, "DBG executing cycle", false},
		{LevelERR, "ERR something failed", true},
		{LevelERR, "ERROR something failed", true},
		{LevelERR, "DBG executing cycle", false},
		{LevelINF | LevelWARN | LevelERR, "DBG executing cycle Component=batch", false},
		{LevelINF | LevelWARN | LevelERR, `"GET http://10.0.20.9:8080/" from 10.0.0.182 - 200`, false},
		{LevelINF | LevelWARN | LevelERR, "INF request handled", true},
		{LevelINF | LevelWARN | LevelERR, "WARN timeout approaching", true},
		{LevelINF | LevelWARN | LevelERR, "ERR connection refused", true},
		{LevelINF | LevelERR, "WARN should not match", false},
		{LevelINF | LevelERR, "INFO should match", true},
	}
	for _, tt := range tests {
		if got := tt.level.Matches(tt.msg); got != tt.want {
			t.Errorf("Level(%s).Matches(%q) = %v, want %v", tt.level, tt.msg, got, tt.want)
		}
	}
}

func TestLevelToggle(t *testing.T) {
	l := LevelAll
	l = l.Toggle()
	if l != LevelINF {
		t.Fatalf("expected INF, got %s", l)
	}
	l = l.Toggle()
	if l != LevelINF|LevelWARN {
		t.Fatalf("expected INF|WARN, got %s", l)
	}
	l = l.Toggle()
	if l != LevelINF|LevelWARN|LevelERR {
		t.Fatalf("expected INF|WARN|ERR, got %s", l)
	}
	l = l.Toggle()
	if l != LevelAll {
		t.Fatalf("expected ALL, got %s", l)
	}
}
