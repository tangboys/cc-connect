//go:build windows

package daemon

import (
	"os"
	"strings"
	"testing"
)

func TestStrictPowerShellStopsOnCmdletErrors(t *testing.T) {
	script := strictPowerShell("Write-Output 'ok'")
	if !strings.HasPrefix(script, "$ErrorActionPreference = 'Stop'\n") {
		t.Fatalf("strictPowerShell() missing stop prelude:\n%s", script)
	}
	if !strings.Contains(script, "Write-Output 'ok'") {
		t.Fatalf("strictPowerShell() missing original script:\n%s", script)
	}
}

func TestBuildWindowsTaskScript(t *testing.T) {
	cfg := Config{
		BinaryPath: `C:\Program Files\cc-connect\cc-connect.exe`,
		WorkDir:    `C:\Users\me\.cc-connect`,
		LogFile:    `C:\Users\me\.cc-connect\logs\cc-connect.log`,
		LogMaxSize: 10 * 1024 * 1024,
		EnvPATH:    `C:\Program Files\nodejs;C:\Users\me\AppData\Local\Programs`,
		EnvExtra: map[string]string{
			"HTTPS_PROXY": "http://127.0.0.1:7890",
			"http_proxy":  "http://127.0.0.1:7890",
		},
	}

	script := buildWindowsTaskScript(cfg)
	for _, want := range []string{
		`$env:CC_LOG_FILE = 'C:\Users\me\.cc-connect\logs\cc-connect.log'`,
		`$env:CC_LOG_MAX_SIZE = '10485760'`,
		`$env:PATH = 'C:\Program Files\nodejs;C:\Users\me\AppData\Local\Programs'`,
		`$env:HTTPS_PROXY = 'http://127.0.0.1:7890'`,
		`$env:http_proxy = 'http://127.0.0.1:7890'`,
		`Set-Location -LiteralPath 'C:\Users\me\.cc-connect'`,
		`while ($true) {`,
		`$binary = 'C:\Program Files\cc-connect\cc-connect.exe'`,
		`Start-Process -FilePath $binary`,
		`-WindowStyle Hidden -PassThru`,
		`Write-SupervisorLog $_.Exception.Message`,
		`if ($exitCode -eq 0) { exit 0 }`,
		`Start-Sleep -Seconds 10`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestWindowsTaskActionRunsHidden(t *testing.T) {
	got := windowsTaskAction(`C:\Users\me\.cc-connect\cc-connect-daemon.ps1`)
	for _, want := range []string{
		`powershell.exe`,
		`-WindowStyle Hidden`,
		`-NoProfile`,
		`-NonInteractive`,
		`-ExecutionPolicy Bypass`,
		`-File "C:\Users\me\.cc-connect\cc-connect-daemon.ps1"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("windowsTaskAction() missing %q: %q", want, got)
		}
	}
}

func TestWindowsTaskCreateUsesLimitedInteractivePrincipal(t *testing.T) {
	orig := runPowerShell
	t.Cleanup(func() { runPowerShell = orig })

	var script string
	runPowerShell = func(s string) (string, error) {
		script = s
		return "", nil
	}

	if err := createWindowsTask(`C:\Users\me\.cc-connect\cc-connect-daemon.ps1`); err != nil {
		t.Fatalf("createWindowsTask() error = %v", err)
	}
	for _, want := range []string{
		`New-ScheduledTaskAction`,
		`Register-ScheduledTask`,
		`-LogonType Interactive`,
		`-RunLevel Limited`,
		`-WindowStyle Hidden`,
		`C:\Users\me\.cc-connect\cc-connect-daemon.ps1`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("create script missing %q:\n%s", want, script)
		}
	}
}

func TestWindowsTaskMatchesActionRequiresExactAction(t *testing.T) {
	orig := runPowerShell
	t.Cleanup(func() { runPowerShell = orig })

	var script string
	runPowerShell = func(s string) (string, error) {
		script = s
		return "true", nil
	}

	if !windowsTaskMatchesAction(`C:\Users\me\.cc-connect\cc-connect-daemon.ps1`) {
		t.Fatal("windowsTaskMatchesAction() = false, want true")
	}
	for _, want := range []string{
		`$expectedArgs = '-WindowStyle Hidden -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "C:\Users\me\.cc-connect\cc-connect-daemon.ps1"'`,
		`$action.Execute -ieq 'powershell.exe'`,
		`$action.Arguments -eq $expectedArgs`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("reuse check script missing %q:\n%s", want, script)
		}
	}
}

func TestPowerShellLiteralEscapesSingleQuotes(t *testing.T) {
	got := powerShellLiteral(`C:\Users\O'Brien\.cc-connect`)
	want := `'C:\Users\O''Brien\.cc-connect'`
	if got != want {
		t.Fatalf("powerShellLiteral() = %q, want %q", got, want)
	}
}

func TestBuildWindowsTaskScript_DropsInvalidEnvName(t *testing.T) {
	cfg := Config{
		BinaryPath: "x", WorkDir: "y", LogFile: "l", LogMaxSize: 1, EnvPATH: "p",
		EnvExtra: map[string]string{"FOO BAR": "v", "OK": "ok"},
	}
	script := buildWindowsTaskScript(cfg)
	if strings.Contains(script, "FOO BAR") {
		t.Errorf("invalid env name leaked: %s", script)
	}
	if !strings.Contains(script, "$env:OK = 'ok'") {
		t.Errorf("valid env missing: %s", script)
	}
}

func TestBuildWindowsTaskScript_DropsEmptyValue(t *testing.T) {
	cfg := Config{
		BinaryPath: "x", WorkDir: "y", LogFile: "l", LogMaxSize: 1, EnvPATH: "p",
		EnvExtra: map[string]string{"EMPTY": "", "OK": "ok"},
	}
	script := buildWindowsTaskScript(cfg)
	if strings.Contains(script, "$env:EMPTY") {
		t.Errorf("empty value should be skipped: %s", script)
	}
}

// Windows reports writable files as 0666 regardless of the requested Unix mode.
// Check the reinstall's actual result rather than asserting unsupported POSIX bits.
func TestSchtasksInstall_ReplacesExistingLauncher(t *testing.T) {
	t.Setenv("USERPROFILE", t.TempDir())

	orig := runPowerShell
	t.Cleanup(func() { runPowerShell = orig })
	runPowerShell = func(script string) (string, error) { return "", nil }

	if err := os.MkdirAll(DefaultDataDir(), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	scriptPath := windowsTaskScriptPath()
	if err := os.WriteFile(scriptPath, []byte("$env:OLD = 'leftover'\r\n"), 0o644); err != nil {
		t.Fatalf("seed legacy script: %v", err)
	}

	mgr := &schtasksManager{}
	cfg := Config{
		BinaryPath: "C:\\cc.exe",
		WorkDir:    t.TempDir(),
		LogFile:    "C:\\cc.log",
		LogMaxSize: 1024,
		EnvPATH:    "C:\\bin",
		EnvExtra:   map[string]string{"CUSTOM_TOKEN": "captured"},
	}
	if err := mgr.Install(cfg); err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if string(data) != buildWindowsTaskScript(cfg) {
		t.Errorf("reinstall did not replace the old launcher")
	}
}

func TestWindowsTaskCreate_KeepsSupervisorRunning(t *testing.T) {
	orig := runPowerShell
	t.Cleanup(func() { runPowerShell = orig })
	var script string
	runPowerShell = func(s string) (string, error) { script = s; return "", nil }
	if err := createWindowsTask(`C:\cc-connect-daemon.ps1`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-ExecutionTimeLimit ([TimeSpan]::Zero)", "-MultipleInstances IgnoreNew", "-AllowStartIfOnBatteries", "-DontStopIfGoingOnBatteries", "-RestartCount 3", "-Settings $settings"} {
		if !strings.Contains(script, want) {
			t.Errorf("task settings missing %q", want)
		}
	}
}

func TestStopWindowsTask_KillsManagedChildTree(t *testing.T) {
	orig := runPowerShell
	t.Cleanup(func() { runPowerShell = orig })
	var script string
	runPowerShell = func(s string) (string, error) { script = s; return "", nil }
	if err := stopWindowsTask(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Stop-ScheduledTask", "taskkill.exe", "/T /F", "$child.StartTime", "$child.Path"} {
		if !strings.Contains(script, want) {
			t.Errorf("stop script missing %q", want)
		}
	}
}
