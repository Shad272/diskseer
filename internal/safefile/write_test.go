package safefile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesCompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	for _, data := range [][]byte{bytes.Repeat([]byte("old report"), 1000), []byte("new report")} {
		if err := Write(path, data); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("saved file = %q, error = %v", got, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}

func TestFailedReplacementPreservesDestinationAndCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "existing")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(kept, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(destination, []byte("replacement")); err == nil {
		t.Fatal("replacing a directory should fail")
	}
	got, err := os.ReadFile(kept)
	if err != nil || string(got) != "keep" {
		t.Fatalf("destination damaged: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}
