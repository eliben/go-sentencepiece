package sentencepiece

import (
	"fmt"
	"os/exec"
	"testing"
)

// TODO: invoke test/sp.py to dump token IDs to stdout, and compare with our
// own, for a bunch of files.
// Requires running inside venv for sp.py to run (skip if some env var not set)
// Uses the same MODELPATH var

func TestVsSentencepiecePython(t *testing.T) {
	// We expect Python3 to be available and for it to successfully load
	// the sentencepiece library.
	if _, err := exec.Command("python3", "-c", "import sentencepiece").Output(); err != nil {
		t.Skip("This test only runs when python3 with sentencepiece is available")
	}

	fmt.Println("foo")
}
