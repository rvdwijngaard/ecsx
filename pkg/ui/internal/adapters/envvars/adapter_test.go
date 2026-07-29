package envvars

import (
	"encoding/json"
	"testing"

	ecstypes "github.com/rvdwijngaard/ecsx/pkg/aws/ecs/types"
)

func TestFormatTable(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "DB_HOST", Value: "localhost"},
		{Name: "PORT", Value: "8080"},
		{Name: "APP_NAME", Value: "myapp"},
	}
	got := formatTable(vars)
	expected := "DB_HOST   localhost\nPORT      8080\nAPP_NAME  myapp"
	if got != expected {
		t.Errorf("formatTable:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestFormatExport(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "DB_HOST", Value: "localhost"},
		{Name: "MSG", Value: `hello "world"`},
	}
	got := formatExport(vars)
	expected := "export DB_HOST=\"localhost\"\nexport MSG=\"hello \\\"world\\\"\""
	if got != expected {
		t.Errorf("formatExport:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestFormatShell(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "SIMPLE", Value: "value"},
		{Name: "SPACED", Value: "hello world"},
	}
	got := formatShell(vars)
	expected := "SIMPLE=value\nSPACED=\"hello world\""
	if got != expected {
		t.Errorf("formatShell:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestFormatDocker(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "DB_HOST", Value: "localhost"},
		{Name: "MSG", Value: "hello world"},
	}
	got := formatDocker(vars)
	expected := "DB_HOST=localhost\nMSG=hello world"
	if got != expected {
		t.Errorf("formatDocker:\ngot:\n%s\nwant:\n%s", got, expected)
	}
}

func TestFormatEmpty(t *testing.T) {
	var vars []ecstypes.EnvVar
	for _, fn := range []func([]ecstypes.EnvVar) string{formatTable, formatExport, formatShell, formatDocker} {
		got := fn(vars)
		if got != "(no environment variables)" {
			t.Errorf("expected empty message, got: %s", got)
		}
	}
	// JSON returns empty object
	if got := formatJSON(vars); got != "{}" {
		t.Errorf("expected {}, got: %s", got)
	}
}

func TestFormatJSON(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "DB_HOST", Value: "localhost"},
		{Name: "PORT", Value: "8080"},
	}
	got := formatJSON(vars)
	// JSON is a map so key order isn't guaranteed; parse and verify
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("invalid JSON: %s", err)
	}
	if m["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST=localhost, got %s", m["DB_HOST"])
	}
	if m["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %s", m["PORT"])
	}
}

func TestSortEnvVars(t *testing.T) {
	vars := []ecstypes.EnvVar{
		{Name: "ZZZ", Value: "last"},
		{Name: "AAA", Value: "first"},
		{Name: "MMM", Value: "middle"},
	}
	sorted := sortEnvVars(vars)
	if sorted[0].Name != "AAA" || sorted[1].Name != "MMM" || sorted[2].Name != "ZZZ" {
		t.Errorf("sortEnvVars not sorted: %v", sorted)
	}
	// Original unchanged
	if vars[0].Name != "ZZZ" {
		t.Error("sortEnvVars mutated original slice")
	}
}

func TestNextFormat(t *testing.T) {
	f := FormatDetail
	f = NextFormat(f)
	if f != FormatExport {
		t.Errorf("expected FormatExport, got %v", f)
	}
	f = NextFormat(f)
	if f != FormatShell {
		t.Errorf("expected FormatShell, got %v", f)
	}
	f = NextFormat(f)
	if f != FormatDocker {
		t.Errorf("expected FormatDocker, got %v", f)
	}
	f = NextFormat(f)
	if f != FormatJSON {
		t.Errorf("expected FormatJSON, got %v", f)
	}
	f = NextFormat(f)
	if f != FormatDetail {
		t.Errorf("expected FormatDetail (wrap), got %v", f)
	}
}
