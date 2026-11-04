package logging_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tosrv/natter/internal/logging"
)

func TestNewLoggerLevels(t *testing.T) {
	var b bytes.Buffer
	l := logging.NewLogger(&b, "warn")
	l.Info("hello-info")
	if b.Len() != 0 {
		t.Fatalf("info written at warn: %s", b.String())
	}
	l.Warn("hello-warn")
	var m map[string]any
	if err := json.Unmarshal(b.Bytes(), &m); err != nil || m["msg"] != "hello-warn" {
		t.Fatalf("bad json %q: %v", b.String(), err)
	}

	b.Reset()
	l = logging.NewLogger(&b, "bogus")
	l.Debug("d")
	l.Info("i")
	if strings.Contains(b.String(), `"d"`) || !strings.Contains(b.String(), `"msg":"i"`) {
		t.Fatalf("bogus should behave as info: %s", b.String())
	}
	b.Reset()
	logging.NewLogger(&b, "debug").Debug("d")
	if !strings.Contains(b.String(), `"msg":"d"`) {
		t.Fatalf("debug not written: %s", b.String())
	}
}
