package logs

import "strings"

type Level uint8

const (
	LevelINF  Level = 1 << iota // 1
	LevelWARN                   // 2
	LevelERR                    // 4
	LevelAll  Level = 0         // show everything
)

func (l Level) String() string {
	if l == LevelAll {
		return "ALL"
	}
	var parts []string
	if l&LevelINF != 0 {
		parts = append(parts, "INF")
	}
	if l&LevelWARN != 0 {
		parts = append(parts, "WARN")
	}
	if l&LevelERR != 0 {
		parts = append(parts, "ERR")
	}
	return strings.Join(parts, "|")
}

// Toggle flips a single level bit. Cycles: ALL → INF → INF|WARN → INF|WARN|ERR → ALL
var toggleOrder = []Level{LevelINF, LevelWARN, LevelERR}

func (l Level) Toggle() Level {
	if l == LevelAll {
		return LevelINF
	}
	for _, bit := range toggleOrder {
		if l&bit == 0 {
			return l | bit
		}
	}
	return LevelAll
}

func (l Level) Matches(msg string) bool {
	if l == LevelAll {
		return true
	}
	upper := strings.ToUpper(msg)
	if l&LevelINF != 0 && (strings.Contains(upper, "INFO") || strings.Contains(upper, "INF")) {
		return true
	}
	if l&LevelWARN != 0 && strings.Contains(upper, "WARN") {
		return true
	}
	if l&LevelERR != 0 && (strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR")) {
		return true
	}
	return false
}

func ParseLevel(s string) Level {
	if s == "" {
		return LevelAll
	}
	var l Level
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' }) {
		switch strings.ToUpper(strings.TrimSpace(part)) {
		case "INF", "INFO":
			l |= LevelINF
		case "WARN", "WARNING":
			l |= LevelWARN
		case "ERR", "ERROR":
			l |= LevelERR
		}
	}
	return l
}
