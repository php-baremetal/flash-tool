package build

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestProgressParsesNinjaAndRendersBar(t *testing.T) {
	var out bytes.Buffer
	p := NewProgress(&out)
	p.Write([]byte("[1/4] Building a\n"))
	p.Write([]byte("[2/4] Building b\n"))
	if p.done != 2 || p.total != 4 {
		t.Fatalf("done/total = %d/%d, want 2/4", p.done, p.total)
	}
	if !strings.Contains(out.String(), "50%") {
		t.Errorf("expected 50%% in output, got %q", out.String())
	}
}

func TestProgressSplitsAcrossWrites(t *testing.T) {
	var out bytes.Buffer
	p := NewProgress(&out)
	p.Write([]byte("[3/"))    // partial line
	p.Write([]byte("4] x\n")) // completes it
	if p.done != 3 || p.total != 4 {
		t.Errorf("done/total = %d/%d, want 3/4", p.done, p.total)
	}
}

func TestProgressDumpRawOnError(t *testing.T) {
	p := NewProgress(io.Discard)
	p.Write([]byte("main.c:10: error: undefined reference to 'boom'\n"))
	var dump bytes.Buffer
	p.DumpRaw(&dump)
	if !strings.Contains(dump.String(), "undefined reference to 'boom'") {
		t.Errorf("raw output not replayed: %q", dump.String())
	}
}
