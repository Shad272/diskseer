//go:build windows

package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/shad272/diskseer/internal/platform"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Discovery reads AppX manifests through the supported PowerShell AppX module.
// It recognises registered execution aliases, not localised display names,
// package versions, installation folders or a particular package publisher ID.
const appxProbe = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding
if(Get-Command Get-AppxPackage -ErrorAction SilentlyContinue){
 Get-AppxPackage | ForEach-Object {
  try {
   $p=$_
   $m=Get-AppxPackageManifest -Package $p.PackageFullName
   foreach($a in $m.SelectNodes("//*[local-name()='Application']")){
    foreach($alias in $a.SelectNodes(".//*[local-name()='ExecutionAlias']")){
     if($alias.Alias -eq 'wt.exe'){
      $file=Join-Path $p.InstallLocation $a.Executable
      if(Test-Path -LiteralPath $file){[Console]::WriteLine($file)}
     }
    }
   }
  } catch {}
 }
}`

// readinessTimeout bounds how long the parent waits for the relaunched child to
// confirm it started. Past it, diskseer stays in the current console, and a
// late child finds the request gone and exits without a second diagnosis. The
// real shell handoff test raises it: it checks what the handoff preserves, not
// how fast a cold PowerShell starts on a busy CI runner.
var readinessTimeout = 10 * time.Second

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 65536 {
		return 0, errors.New("discovery output limit")
	}
	return b.Buffer.Write(p)
}

func Discover(c platform.Capabilities) []Candidate {
	found, _ := DiscoverDetailed(c)
	return found
}

func DiscoverDetailed(c platform.Capabilities) ([]Candidate, []string) {
	var found []Candidate
	var notes []string
	for _, kind := range []string{"wt", "pwsh", "powershell", "cmd"} {
		if kind == "wt" && !c.CanUseTerminal() || kind == "pwsh" && c.Major < 10 {
			continue
		}
		if p, err := exec.LookPath(kind + ".exe"); err == nil {
			found = append(found, Candidate{kind, p})
			notes = append(notes, kind+": found on PATH")
		}
	}
	ps := platform.SystemExecutable(filepath.Join("WindowsPowerShell", "v1.0", "powershell.exe"))
	if _, err := os.Stat(ps); err == nil {
		found = append(found, Candidate{"powershell", ps})
	}
	cmdPath := platform.SystemExecutable("cmd.exe")
	if _, err := os.Stat(cmdPath); err == nil {
		found = append(found, Candidate{"cmd", cmdPath})
	}
	if !c.CanUseTerminal() || ps == "" {
		return found, append(notes, "AppX discovery skipped: terminal capabilities unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ps, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", appxProbe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out limitedOutput
	cmd.Stdout = &out
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if err != nil {
		notes = append(notes, "AppX discovery unavailable or timed out; keeping shell fallbacks")
	}
	registered := ManifestCandidates(out.String())
	if len(registered) > 0 {
		notes = append(notes, "Windows Terminal found through registered AppX execution aliases")
	}
	return append(found, registered...), notes
}

func ManifestCandidates(output string) []Candidate {
	var out []Candidate
	for _, line := range strings.Split(output, "\n") {
		p := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if filepath.IsAbs(p) && strings.EqualFold(filepath.Ext(p), ".exe") {
			out = append(out, Candidate{"wt", p})
		}
	}
	return out
}

// Only constant bootstrap source enters a shell. User paths and arguments are
// inherited via environment and a private JSON file, including %, !, &, quotes
// and semicolons. cmd expansion is performed once, with delayed expansion off.
func command(c Candidate) []string {
	switch c.Kind {
	case "wt":
		return []string{"-w", "new", "new-tab", "--inheritEnvironment", "--", platform.SystemExecutable(filepath.Join("WindowsPowerShell", "v1.0", "powershell.exe")), "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `& $env:DISKSEER_EXECUTABLE --terminal-child`}
	case "pwsh", "powershell":
		return []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `& $env:DISKSEER_EXECUTABLE --terminal-child`}
	default:
		return []string{"/d", "/v:off", "/s", "/c", `""%DISKSEER_EXECUTABLE%" --terminal-child"`}
	}
}

// Launch retains the parent until the acknowledged child publishes its exit
// code. This preserves exit status even when wt.exe itself exits immediately.
func Launch(candidates []Candidate, args []string, own bool, log func(string)) (bool, int) {
	return launch(candidates, args, own, log, false)
}

func launch(candidates []Candidate, args []string, own bool, log func(string), hidden bool) (bool, int) {
	exe, err := os.Executable()
	if err != nil {
		return false, 0
	}
	return launchExecutable(exe, candidates, args, own, log, hidden)
}

func launchExecutable(exe string, candidates []Candidate, args []string, own bool, log func(string), hidden bool) (bool, int) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, 0
	}
	raw, err := json.Marshal(payload{args, cwd})
	if err != nil {
		return false, 0
	}
	var activeDir string
	var child syscall.Handle
	ok := Try(candidates, func(c Candidate) error {
		dir, err := os.MkdirTemp("", "diskseer-terminal-")
		if err != nil {
			return err
		}
		keep := false
		defer func() {
			if !keep {
				os.RemoveAll(dir)
			}
		}()
		if err := os.WriteFile(filepath.Join(dir, "request.json"), raw, 0o600); err != nil {
			return err
		}
		env := []string{}
		for _, entry := range os.Environ() {
			key := strings.SplitN(entry, "=", 2)[0]
			if !strings.EqualFold(key, handoffEnv) && !strings.EqualFold(key, executableEnv) {
				env = append(env, entry)
			}
		}
		env = append(env, handoffEnv+"="+dir, executableEnv+"="+exe)
		done, kill, err := startProcess(c, env, cwd, hidden)
		if err != nil {
			return err
		}
		defer func() {
			if !keep {
				kill()
			}
		}()
		deadline := time.Now().Add(readinessTimeout)
		for time.Now().Before(deadline) {
			if ready, err := os.ReadFile(filepath.Join(dir, "ready")); err == nil {
				pid, err := strconv.Atoi(string(ready))
				if err != nil {
					return err
				}
				h, err := syscall.OpenProcess(0x100000, false, uint32(pid))
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "go"), []byte("go"), 0o600); err != nil {
					syscall.CloseHandle(h)
					return err
				}
				keep, activeDir, child = true, dir, h
				return nil
			}
			select {
			case err := <-done:
				if c.Kind != "wt" || err != nil {
					return errors.New("terminal exited before child readiness")
				}
			default:
			}
			time.Sleep(25 * time.Millisecond)
		}
		return errors.New("terminal readiness timeout")
	}, log)
	if !ok {
		return false, 0
	}
	defer os.RemoveAll(activeDir)
	defer syscall.CloseHandle(child)
	if own {
		hideOwnedConsole()
	}
	for {
		if data, err := os.ReadFile(filepath.Join(activeDir, "exit")); err == nil {
			if n, err := strconv.Atoi(string(data)); err == nil {
				return true, n
			}
		}
		if status, err := syscall.WaitForSingleObject(child, 0); err != nil || status == 0 {
			// The exit file may have been published between the read above and
			// the kernel signalling termination.
			if data, err := os.ReadFile(filepath.Join(activeDir, "exit")); err == nil {
				if n, err := strconv.Atoi(string(data)); err == nil {
					return true, n
				}
			}
			return true, 3
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func hideOwnedConsole() {
	k := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
	u := syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	if k.Find() == nil && u.Find() == nil {
		h, _, _ := k.Call()
		if h != 0 {
			u.Call(h, 0)
		}
	}
}
