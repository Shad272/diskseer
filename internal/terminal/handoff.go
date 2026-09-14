package terminal

import (
	"encoding/json"
	"errors"
	"github.com/shad272/diskseer/internal/safefile"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const childFlag = "--terminal-child"
const handoffEnv = "DISKSEER_HANDOFF"
const executableEnv = "DISKSEER_EXECUTABLE"

type payload struct {
	Args      []string
	Directory string
}

// Receive restores user arguments from data, never from shell source. A two
// phase handshake ensures a late child cannot start after a fallback did.
func Receive() (bool, func(int), error) {
	if len(os.Args) != 2 || os.Args[1] != childFlag {
		return false, func(int) {}, nil
	}
	dir := os.Getenv(handoffEnv)
	if !filepath.IsAbs(dir) {
		return true, nil, errors.New("invalid terminal handoff")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "request.json"))
	if err != nil || len(raw) > 1<<20 {
		return true, nil, errors.New("terminal request unavailable")
	}
	var request payload
	if err := json.Unmarshal(raw, &request); err != nil {
		return true, nil, err
	}
	if err := os.Chdir(request.Directory); err != nil {
		return true, nil, err
	}
	if err := safefile.Write(filepath.Join(dir, "ready"), []byte(strconv.Itoa(os.Getpid()))); err != nil {
		return true, nil, err
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go")); err == nil {
			break
		}
		if _, err := os.Stat(dir); err != nil || time.Now().After(deadline) {
			return true, nil, errors.New("terminal handoff expired")
		}
		time.Sleep(25 * time.Millisecond)
	}
	os.Args = append([]string{os.Args[0]}, request.Args...)
	os.Unsetenv(handoffEnv)
	os.Unsetenv(executableEnv)
	return true, func(code int) { _ = safefile.Write(filepath.Join(dir, "exit"), []byte(strconv.Itoa(code))) }, nil
}
