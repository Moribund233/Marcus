package sandbox

import (
	"strings"
	"testing"
)

func TestBuildEnv(t *testing.T) {
	env := buildEnv()
	var pathVal string
	for _, e := range env {
		if strings.EqualFold(strings.SplitN(e, "=", 2)[0], "PATH") {
			pathVal = strings.SplitN(e, "=", 2)[1]
			break
		}
	}
	if pathVal == "" {
		t.Fatal("PATH not found in buildEnv output")
	}
}

func TestResolvePath(t *testing.T) {
	path, err := resolvePath("marcus-textstats")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if !strings.HasSuffix(path, "marcus-textstats") && !strings.HasSuffix(path, "marcus-textstats.exe") {
		t.Fatalf("unexpected path: %s", path)
	}
}
