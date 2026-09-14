//go:build windows

package terminal

import (
	"encoding/json"
	"github.com/shad272/diskseer/internal/platform"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("DISKSEER_TEST_CHILD") == "1" && len(os.Args) == 2 && os.Args[1] == childFlag {
		_, finish, err := Receive()
		if err != nil {
			os.Exit(91)
		}
		caps := platform.Detect()
		if !caps.Console || !caps.InputConsole {
			os.Exit(93)
		}
		if os.Getenv("DISKSEER_TEST_VALUE") != "è & % ! ;" {
			os.Exit(94)
		}
		cwd, _ := os.Getwd()
		data, _ := json.Marshal(payload{os.Args[1:], cwd})
		if err := os.WriteFile(os.Getenv("DISKSEER_TEST_OUTPUT"), data, 0o600); err != nil {
			os.Exit(92)
		}
		finish(2)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestRealShellHandoffPreservesArgumentsDirectoryAndExit(t *testing.T) {
	if os.Getenv("DISKSEER_LAUNCH_TESTS") == "" {
		t.Skip("opt-in hidden native shell integration test")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "spazi è 日本語 & % ! ;")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	t.Setenv("DISKSEER_TEST_CHILD", "1")
	t.Setenv("DISKSEER_TEST_VALUE", "è & % ! ;")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(dir, "disk seer è 日本語 & % !.exe")
	if err := os.WriteFile(copied, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	args := []string{"--customer", "Anna \"detta Ann\"", "", "末尾", `C:\spazi è\`, "%PATH% ! & | < > ; $(exit) `test`"}
	for _, c := range []Candidate{{"cmd", platform.SystemExecutable("cmd.exe")}, {"powershell", platform.SystemExecutable(filepath.Join("WindowsPowerShell", "v1.0", "powershell.exe"))}} {
		t.Run(c.Kind, func(t *testing.T) {
			output := filepath.Join(dir, c.Kind+".json")
			t.Setenv("DISKSEER_TEST_OUTPUT", output)
			ok, code := launchExecutable(copied, []Candidate{{"wt", filepath.Join(dir, "missing.exe")}, c}, args, false, func(s string) { t.Log(s) }, true)
			if !ok || code != 2 {
				t.Fatalf("launch=%v code=%d", ok, code)
			}
			raw, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var got payload
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Args, args) || got.Directory != dir {
				t.Fatalf("handoff changed data: %+v", got)
			}
		})
	}
}

func TestManifestCandidatesSupportVersionedPreviewPaths(t *testing.T) {
	out := ManifestCandidates("C:\\Program Files\\WindowsApps\\example-preview_2026\\WindowsTerminal.exe\r\nnoise\r\nrelative.exe\n")
	if len(out) != 1 || out[0].Kind != "wt" {
		t.Fatal(out)
	}
}
