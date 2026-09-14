//go:build windows

package terminal

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// A new console needs its own standard handles. Leave STARTF_USESTDHANDLES
// unset so Windows initialises them instead of inheriting NUL from os/exec.
func startProcess(c Candidate, env []string, cwd string, hidden bool) (<-chan error, func(), error) {
	app, err := syscall.UTF16PtrFromString(c.Path)
	if err != nil {
		return nil, nil, err
	}
	args := append([]string{c.Path}, command(c)...)
	for i := range args {
		args[i] = syscall.EscapeArg(args[i])
	}
	line := strings.Join(args, " ")
	if c.Kind == "cmd" {
		line = syscall.EscapeArg(c.Path) + ` /d /v:off /s /c ""%DISKSEER_EXECUTABLE%" --terminal-child"`
	}
	cmd, err := syscall.UTF16PtrFromString(line)
	if err != nil {
		return nil, nil, err
	}
	dir, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(env)
	block := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))
	startup := syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(syscall.StartupInfo{}))}
	if hidden {
		startup.Flags = 1
		startup.ShowWindow = 0
	}
	var pi syscall.ProcessInformation
	if err := syscall.CreateProcess(app, cmd, nil, nil, false, 0x10|0x400, &block[0], dir, &startup, &pi); err != nil {
		return nil, nil, err
	}
	syscall.CloseHandle(pi.Thread)
	var mu sync.Mutex
	closed := false
	done := make(chan error, 1)
	go func() {
		_, err := syscall.WaitForSingleObject(pi.Process, syscall.INFINITE)
		var code uint32
		if err == nil {
			err = syscall.GetExitCodeProcess(pi.Process, &code)
			if err == nil && code != 0 {
				err = fmt.Errorf("terminal exit %d", code)
			}
		}
		mu.Lock()
		syscall.CloseHandle(pi.Process)
		closed = true
		mu.Unlock()
		done <- err
	}()
	kill := func() {
		mu.Lock()
		defer mu.Unlock()
		if !closed {
			_ = syscall.TerminateProcess(pi.Process, 3)
		}
	}
	return done, kill, nil
}
