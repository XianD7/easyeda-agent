package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zhoushoujianwork/easyeda-agent/internal/autostart"
	"github.com/zhoushoujianwork/easyeda-agent/internal/selfupdate"
	"github.com/zhoushoujianwork/easyeda-agent/internal/version"
)

// daemonReadyTimeout bounds how long enable, --detach, restart and update wait
// for the daemon to answer /health.
const daemonReadyTimeout = 15 * time.Second

func newDaemonAutostartCmd(cfg *appConfig, stdout io.Writer) *cobra.Command {
	c := &cobra.Command{
		Use:   "autostart",
		Short: "Start the daemon at login (launchd / systemd --user / Windows Run)",
		Long: `Register the daemon to start at login, without administrator rights:
  macOS    LaunchAgent; launchd restarts the daemon if it exits
  Linux    systemd --user unit; systemd restarts the daemon if it exits
  Windows  HKCU Run entry running ` + "`daemon start --detach`" + ` at login

While launchd/systemd manage the daemon, ` + "`daemon start|stop|restart`" + ` in a terminal
are handed to them instead of competing for the port. ` + "`easyeda update`" + ` restarts
a registered daemon after replacing the binary.`,
		Args: cobra.NoArgs,
	}
	c.AddCommand(
		newDaemonAutostartEnableCmd(cfg, stdout),
		newDaemonAutostartDisableCmd(cfg, stdout),
		newDaemonAutostartStatusCmd(cfg, stdout),
	)
	return c
}

func newDaemonAutostartEnableCmd(cfg *appConfig, stdout io.Writer) *cobra.Command {
	var autosaveDebounce time.Duration
	var autoUpdateSkill bool
	c := &cobra.Command{
		Use:   "enable",
		Short: "Register the daemon to start at login and start it now",
		Long: `Register this easyeda binary to run ` + "`daemon start`" + ` at login and start it now.
Start flags given here (and non-default --host / --ports) are written into the
registration. launchd/systemd units also record CLAUDE_CONFIG_DIR, CODEX_HOME
and the EASYEDA_* path variables set in this shell; on Windows the daemon
inherits the user environment. Enabling again replaces the registration.

A daemon already running in a terminal is replaced on the same port. On Linux
the unit starts at login; ` + "`loginctl enable-linger`" + ` starts it at boot instead.`,
		Args: cobra.NoArgs,
		Example: `  easyeda daemon autostart enable
  easyeda daemon autostart enable --autosave-debounce 5s
  easyeda daemon autostart enable --auto-update-skill=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := selfupdate.CurrentBinaryPath()
			if err != nil {
				return fmt.Errorf("resolve easyeda binary: %w", err)
			}
			daemonArgs := []string{"daemon", "start"}
			if cfg.host != defaultHost {
				daemonArgs = append(daemonArgs, "--host", cfg.host)
			}
			if cfg.ports != fmt.Sprintf("%d-%d", defaultPortStart, defaultPortEnd) {
				daemonArgs = append(daemonArgs, "--ports", cfg.ports)
			}
			if cmd.Flags().Changed("autosave-debounce") {
				daemonArgs = append(daemonArgs, "--autosave-debounce", autosaveDebounce.String())
			}
			if cmd.Flags().Changed("auto-update-skill") {
				daemonArgs = append(daemonArgs, fmt.Sprintf("--auto-update-skill=%t", autoUpdateSkill))
			}
			spec := autostart.Spec{
				Executable: exe,
				Arguments:  daemonArgs,
				Env:        autostart.CapturedEnv(),
				LogDir:     daemonLogDir(),
			}
			if err := autostart.Enable(spec); err != nil {
				return err
			}
			st := autostart.Query(spec.LogDir)
			fmt.Fprintf(stdout, "Daemon autostart enabled (%s): %s %s\n", st.Manager, exe, strings.Join(daemonArgs, " "))
			return reportDaemonReady(cfg, stdout, version.Version, st.Logs)
		},
	}
	c.Flags().DurationVar(&autosaveDebounce, "autosave-debounce", 3*time.Second,
		"passed to `daemon start`: autosave a window this long after its last mutating action (0 = disable)")
	c.Flags().BoolVar(&autoUpdateSkill, "auto-update-skill", true,
		"passed to `daemon start`: on startup, sync installed skill dirs to the daemon's release")
	return c
}

func newDaemonAutostartDisableCmd(cfg *appConfig, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "disable",
		Short:   "Remove the login registration and stop the daemon",
		Long:    "Remove the login registration and stop the daemon it started. Save documents first. Afterwards run `easyeda daemon start` in a terminal when needed. Disabling when autostart is not enabled only stops nothing.",
		Args:    cobra.NoArgs,
		Example: "  easyeda daemon autostart disable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !autostart.Enabled() {
				fmt.Fprintln(stdout, "Daemon autostart is not enabled.")
				return nil
			}
			if err := autostart.Disable(); err != nil {
				return err
			}
			// launchd/systemd stop the daemon they own; a Run-entry daemon is ours to stop.
			if !autostart.Supervised() {
				port, _, err := cfg.portRange()
				if err != nil {
					return err
				}
				if err := stopLocalDaemon(cfg.host, port, stdout); err != nil {
					return err
				}
			}
			fmt.Fprintln(stdout, "Daemon autostart disabled; the daemon it started is stopped.")
			return nil
		},
	}
}

func newDaemonAutostartStatusCmd(cfg *appConfig, stdout io.Writer) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:     "status",
		Short:   "Report the login registration and whether the daemon is serving",
		Args:    cobra.NoArgs,
		Example: "  easyeda daemon autostart status\n  easyeda daemon autostart status --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			st := autostart.Query(daemonLogDir())
			ver, pid, serving := daemonIdentity(cfg)
			if jsonOut {
				emitJSON(stdout, struct {
					autostart.Status
					Serving bool   `json:"serving"`
					Version string `json:"daemonVersion,omitempty"`
					PID     int    `json:"daemonPid,omitempty"`
				}{st, serving, ver, pid})
				return nil
			}
			state := "disabled"
			switch {
			case !st.Supported:
				state = "unsupported"
			case st.Enabled && st.Supervised:
				state = "enabled, restarted by " + st.Manager + " if it fails"
			case st.Enabled:
				state = "enabled, started at login"
			}
			daemonState := "not serving"
			if serving {
				daemonState = fmt.Sprintf("%s serving (pid %d)", ver, pid)
			}
			fmt.Fprintf(stdout, "  %-10s %s (%s)\n", "autostart", state, st.Manager)
			fmt.Fprintf(stdout, "  %-10s %s\n", "daemon", daemonState)
			if st.Logs != "" {
				fmt.Fprintf(stdout, "  %-10s %s\n", "logs", st.Logs)
			}
			if st.Reason != "" {
				fmt.Fprintf(stdout, "  %-10s %s\n", "note", st.Reason)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "emit JSON")
	return c
}

// daemonLogDir holds the stdout/stderr files of a daemon that runs without a
// terminal (launchd agent, --detach), next to daemon.pid.
func daemonLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".easyeda-agent", "logs")
}

// delegateToManager hands `daemon start|stop|restart` from a terminal to
// launchd/systemd while they own the daemon: a terminal daemon would otherwise
// be replaced by the one they restart. It reports whether it handled the call.
func delegateToManager(cmd *cobra.Command, cfg *appConfig, stdout io.Writer) (bool, error) {
	if !autostart.Supervised() || !autostart.Interactive() || !autostart.Enabled() {
		return false, nil
	}
	action := cmd.Name()
	manager := autostart.Query("").Manager
	for _, name := range []string{"autosave-debounce", "auto-update-skill", "detach"} {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			fmt.Fprintf(stdout, "Note: --%s is ignored; the daemon keeps the flags it was registered with (re-run `easyeda daemon autostart enable` to change them).\n", name)
		}
	}
	var err error
	switch action {
	case "start":
		err = autostart.Start()
	case "stop":
		err = autostart.Stop()
	default:
		err = autostart.Restart()
	}
	if err != nil {
		return true, fmt.Errorf("%s via %s: %w", action, manager, err)
	}
	if action == "stop" {
		fmt.Fprintf(stdout, "Daemon stopped by %s; it starts again at next login or with `easyeda daemon start`.\n", manager)
		return true, nil
	}
	fmt.Fprintf(stdout, "Daemon is managed by %s (`easyeda daemon autostart`); %s handed to it.\n", manager, action)
	return true, reportDaemonReady(cfg, stdout, "", autostart.Query(daemonLogDir()).Logs)
}

// daemonIdentity reads version and PID from the daemon on the fixed port.
func daemonIdentity(cfg *appConfig) (string, int, bool) {
	port, _, err := cfg.portRange()
	if err != nil {
		return "", 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	scan := scanHealth(ctx, hostPortOptions{host: cfg.host, portStart: port, portEnd: port})
	if scan.Found == nil {
		return "", 0, false
	}
	var h struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	if json.Unmarshal(scan.Found.Raw, &h) != nil {
		return "", 0, false
	}
	return h.Version, h.PID, true
}

// waitDaemon polls until the daemon answers /health and accept(version, pid)
// holds, the timeout elapses or exited reports that the process is gone.
func waitDaemon(cfg *appConfig, accept func(string, int) bool, exited <-chan error) (string, bool, error) {
	deadline := time.Now().Add(daemonReadyTimeout)
	seen := ""
	for {
		if ver, pid, ok := daemonIdentity(cfg); ok {
			seen = ver
			if accept(ver, pid) {
				return ver, true, nil
			}
		}
		select {
		case err := <-exited:
			return seen, false, fmt.Errorf("daemon exited before serving: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			return seen, false, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// reportDaemonReady waits for the daemon to serve want (any version when
// empty) and prints where it logs.
func reportDaemonReady(cfg *appConfig, stdout io.Writer, want string, logs string) error {
	want = strings.TrimPrefix(want, "v")
	got, ok, _ := waitDaemon(cfg, func(v string, _ int) bool {
		return want == "" || strings.TrimPrefix(v, "v") == want
	}, nil)
	if !ok {
		if got != "" {
			return fmt.Errorf("daemon answers as %s, expected %s after %s; logs: %s", got, want, daemonReadyTimeout, logs)
		}
		return fmt.Errorf("daemon did not answer /health within %s; logs: %s", daemonReadyTimeout, logs)
	}
	fmt.Fprintf(stdout, "Daemon %s is serving; logs: %s\n", got, logs)
	return nil
}

// startDetached runs this same `daemon start` command line, minus --detach, as
// a background process with output appended to the log files, and returns once
// that process answers /health.
func startDetached(cfg *appConfig, stdout io.Writer) error {
	exe, err := selfupdate.CurrentBinaryPath()
	if err != nil {
		return fmt.Errorf("resolve easyeda binary: %w", err)
	}
	logDir := daemonLogDir()
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	outPath, errPath := autostart.LogFiles(logDir)
	outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer outFile.Close()
	errFile, err := os.OpenFile(errPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer errFile.Close()

	child := exec.Command(exe, withoutDetach(os.Args[1:])...)
	child.Stdout, child.Stderr = outFile, errFile
	child.SysProcAttr = detachedProcAttr()
	if err := child.Start(); err != nil {
		return fmt.Errorf("start background daemon: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	pid := child.Process.Pid
	ver, ok, err := waitDaemon(cfg, func(_ string, p int) bool { return p == pid }, exited)
	logs := outPath + ", " + errPath
	if err != nil {
		return fmt.Errorf("%w; logs: %s", err, logs)
	}
	if !ok {
		return fmt.Errorf("background daemon pid %d did not answer /health within %s; logs: %s", pid, daemonReadyTimeout, logs)
	}
	fmt.Fprintf(stdout, "Daemon %s started in the background (pid %d); logs: %s\n", ver, pid, logs)
	return nil
}

func withoutDetach(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--detach" || a == "--detach=true" {
			continue
		}
		out = append(out, a)
	}
	return out
}
