// Package safefile saves complete files before replacing their previous version.
package safefile

import (
	"os"
	"path/filepath"
)

// Write keeps the previous file intact if writing or replacing it fails.
// The temporary file is in the destination directory, so replacement never
// crosses filesystems. In particular, live reports are not truncated in place
// while a browser may be reading them.
func Write(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".diskseer-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
