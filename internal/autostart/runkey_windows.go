package autostart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// runKeyPath is the per-user list of commands Explorer runs at login; writing
// it needs no administrator rights.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func init() { platform = runKeyBackend{} }

// runKeyBackend registers `easyeda daemon start --detach` in HKCU Run. Nothing
// supervises the daemon afterwards, so the terminal lifecycle commands keep
// working unchanged and a restart simply runs the registered command again:
// `daemon start` replaces the easyeda daemon already holding the port.
type runKeyBackend struct{}

func (runKeyBackend) manager() string  { return "windows-run" }
func (runKeyBackend) supervised() bool { return false }

func (runKeyBackend) logHint(logDir string) string {
	out, errLog := LogFiles(logDir)
	return out + ", " + errLog
}

// commandLine is the Run entry value: the arguments plus --detach, so the
// login-time console exits as soon as the daemon is serving.
func commandLine(spec Spec) string {
	args := append([]string{spec.Executable}, spec.Arguments...)
	return windows.ComposeCommandLine(append(args, "--detach"))
}

func (b runKeyBackend) enable(spec Spec) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open HKCU\\%s: %w", runKeyPath, err)
	}
	defer key.Close()
	if err := key.SetStringValue(Name, commandLine(spec)); err != nil {
		return fmt.Errorf("write HKCU\\%s\\%s: %w", runKeyPath, Name, err)
	}
	return b.start()
}

func (runKeyBackend) disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open HKCU\\%s: %w", runKeyPath, err)
	}
	defer key.Close()
	if err := key.DeleteValue(Name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove HKCU\\%s\\%s: %w", runKeyPath, Name, err)
	}
	return nil
}

func registered() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer key.Close()
	v, _, err := key.GetStringValue(Name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return v, err
}

func (runKeyBackend) enabled() (bool, error) {
	v, err := registered()
	return v != "", err
}

// running is not tracked: no process manager owns the daemon on Windows.
func (runKeyBackend) running() (*bool, error) { return nil, nil }

// start runs the registered command line exactly as Explorer would at login.
func (runKeyBackend) start() error {
	v, err := registered()
	if err != nil {
		return err
	}
	if v == "" {
		return errors.New("daemon autostart is not enabled")
	}
	args, err := windows.DecomposeCommandLine(v)
	if err != nil || len(args) == 0 {
		return fmt.Errorf("parse HKCU\\%s\\%s %q: %v", runKeyPath, Name, v, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", v, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (runKeyBackend) stop() error {
	return errors.New("the Windows Run entry does not supervise the daemon; stop it with `easyeda daemon stop`")
}

func (b runKeyBackend) restart() error { return b.start() }

func (runKeyBackend) executable() (string, error) {
	v, err := registered()
	if err != nil || v == "" {
		return "", err
	}
	args, err := windows.DecomposeCommandLine(v)
	if err != nil || len(args) == 0 {
		return "", fmt.Errorf("parse HKCU\\%s\\%s %q: %v", runKeyPath, Name, v, err)
	}
	return args[0], nil
}
