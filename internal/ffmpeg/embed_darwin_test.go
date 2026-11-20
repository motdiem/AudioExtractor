package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// prepareBinary must turn an unsigned executable into one that macOS (Apple
// Silicon included) will run.
func TestPrepareBinarySignsUnsignedExecutable(t *testing.T) {
	src, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Skip(err)
	}
	p := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(p, src, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/codesign", "--remove-signature", p).CombinedOutput(); err != nil {
		t.Skipf("cannot strip signature: %v %s", err, out)
	}
	if exec.Command("/usr/bin/codesign", "--verify", p).Run() == nil {
		t.Fatal("precondition: file should be unsigned")
	}

	if err := prepareBinary(p); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", p).CombinedOutput(); err != nil {
		t.Fatalf("not signed after prepare: %v %s", err, out)
	}
	if err := exec.Command(p).Run(); err != nil {
		t.Fatalf("signed binary does not run: %v", err)
	}
}
