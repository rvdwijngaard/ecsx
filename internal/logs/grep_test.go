package logs

import (
	"regexp"
	"testing"
)

func TestGrepRegexFiltering(t *testing.T) {
	tests := []struct {
		pattern string
		msg     string
		want    bool
	}{
		{"ERROR|WARN", "2026-04-14 ERR something broke", false},
		{"ERROR|WARN", "2026-04-14 ERROR something broke", true},
		{"ERROR|WARN", "2026-04-14 WARN low memory", true},
		{"ERROR|WARN", "2026-04-14 INFO all good", false},
		{"ERROR|WARN", "2026-04-14 DBG executing cycle", false},
		{`duration=[0-9]{4,}ms`, "request duration=1234ms slow", true},
		{`duration=[0-9]{4,}ms`, "request duration=12ms fast", false},
		{`(?i)error`, "Something Error happened", true},
		{`(?i)error`, "all good", false},
		{`user=(alice|bob)`, "user=alice logged in", true},
		{`user=(alice|bob)`, "user=charlie logged in", false},
	}
	for _, tt := range tests {
		re := regexp.MustCompile(tt.pattern)
		if got := re.MatchString(tt.msg); got != tt.want {
			t.Errorf("grep(%q).Match(%q) = %v, want %v", tt.pattern, tt.msg, got, tt.want)
		}
	}
}

func TestGrepInvalidRegex(t *testing.T) {
	_, err := regexp.Compile("[invalid")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
