package build

import (
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	in := `# a comment
APP_NAME=esp32
export EXPORTED=yes
QUOTED="hello world"
SINGLE='raw $x value'
ESCAPED="line1\nline2"
SPACED   =   trimmed
9BAD=skip
bad key=skip
EMPTYVAL=

TRAILING=ok
`
	got := ParseDotEnv(in)
	want := map[string]string{
		"APP_NAME": "esp32",
		"EXPORTED": "yes",
		"QUOTED":   "hello world",
		"SINGLE":   "raw $x value",
		"ESCAPED":  "line1\nline2",
		"SPACED":   "trimmed",
		"EMPTYVAL": "",
		"TRAILING": "ok",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs, want %d: %+v", len(got), len(want), got)
	}
	for _, p := range got {
		w, ok := want[p.Key]
		if !ok {
			t.Errorf("unexpected key %q", p.Key)
			continue
		}
		if p.Value != w {
			t.Errorf("%s = %q, want %q", p.Key, p.Value, w)
		}
	}
}

func TestEnvSourceCEscaping(t *testing.T) {
	src := envSourceC([]EnvPair{
		{Key: "K", Value: `a"b\c`},
		{Key: "N", Value: "tab\there"},
	})
	if !strings.Contains(src, `"K", "a\"b\\c",`) {
		t.Errorf("quote/backslash not escaped:\n%s", src)
	}
	if !strings.Contains(src, `"N", "tab\there",`) {
		t.Errorf("tab not escaped:\n%s", src)
	}
	if !strings.Contains(src, "php_esp32_env_count = 2;") {
		t.Errorf("count wrong:\n%s", src)
	}
}

func TestEnvSourceCEmpty(t *testing.T) {
	src := envSourceC(nil)
	if !strings.Contains(src, "php_esp32_env_count = 0;") {
		t.Errorf("empty count wrong:\n%s", src)
	}
	if !strings.Contains(src, "(const char *)0") {
		t.Errorf("empty table needs a dummy element:\n%s", src)
	}
}
