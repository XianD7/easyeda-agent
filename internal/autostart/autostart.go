// Package autostart registers the easyeda daemon to start at user login, with
// the per-user mechanism each platform provides without administrator rights:
//
//   - macOS: a LaunchAgent, kept alive by launchd
//   - Linux: a systemd --user unit, restarted by systemd
//   - Windows: an HKCU Run entry that runs `daemon start --detach` at login
//
// launchd and systemd units are written and controlled through
// github.com/kardianos/service. Its Windows backend registers a SCM service,
// which needs administrator rights and runs as SYSTEM, where the user's skill
// dirs are not visible, so Windows uses the Run entry instead (as mutagen does).
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Name is the launchd label, systemd unit and Run entry name.
const Name = "easyeda-agent"

// ManagedEnv marks a daemon started by launchd/systemd, so its own startup
// is not mistaken for a terminal command competing with the service manager.
const ManagedEnv = "EASYEDA_DAEMON_MANAGED"

// PassEnv lists the variables a daemon started from a terminal would inherit
// and that change where it reads or writes. launchd and systemd start the
// daemon with a minimal environment, so their values at enable time are written
// into the unit. A Run entry inherits the user's environment from Explorer.
// Credentials are never copied.
var PassEnv = []string{
	"CLAUDE_CONFIG_DIR",
	"CODEX_HOME",
	"EASYEDA_SKILLS_DIR",
	"EASYEDA_SKILL_PRESERVE",
	"EASYEDA_GITHUB_PROXY",
	"EASYEDA_AUDIT_DIR",
	"EASYEDA_WORKFLOW_DIR",
}

// ErrUnsupported is returned where no per-user login mechanism is available
// (for example Linux without systemd).
var ErrUnsupported = errors.New("daemon autostart is not supported on this platform")

// Spec is what the registration runs at login.
type Spec struct {
	Executable string            // absolute path of the easyeda binary
	Arguments  []string          // `daemon start` and its flags
	Env        map[string]string // see PassEnv; launchd/systemd only
	LogDir     string            // daemon stdout/stderr files (systemd uses the journal)
}

// Status is the registration state reported by `daemon autostart status`.
type Status struct {
	Supported  bool   `json:"supported"`
	Manager    string `json:"manager"`    // launchd | systemd-user | windows-run | the unsupported platform
	Enabled    bool   `json:"enabled"`    // registered to start at login
	Supervised bool   `json:"supervised"` // the manager restarts the daemon if it exits
	Running    *bool  `json:"running,omitempty"`
	Logs       string `json:"logs,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// backend is one platform mechanism.
type backend interface {
	manager() string
	// supervised reports whether the manager keeps the daemon alive, so
	// start/stop/restart must go through it.
	supervised() bool
	enable(Spec) error
	disable() error
	enabled() (bool, error)
	// running is nil when the manager does not track the process.
	running() (*bool, error)
	start() error
	stop() error
	restart() error
	logHint(logDir string) string
}

// platform is chosen at init: the Windows Run backend, or a kardianos
// service backend when the platform has a per-user service manager.
var platform backend

func current() (backend, error) {
	if platform == nil {
		return nil, fmt.Errorf("%w (%s)", ErrUnsupported, unsupportedName())
	}
	return platform, nil
}

// Supported reports whether this platform can register the daemon.
func Supported() bool { return platform != nil }

// Supervised reports whether the registered daemon is kept alive by a service
// manager (launchd/systemd), which then owns start/stop/restart.
func Supervised() bool { return platform != nil && platform.supervised() }

// Interactive reports whether this process was started from a terminal rather
// than by launchd/systemd.
func Interactive() bool { return os.Getenv(ManagedEnv) == "" }

// Enable registers the daemon to start at login and starts it. Enabling again
// replaces the registration, so changed flags or a moved binary take effect.
func Enable(spec Spec) error {
	b, err := current()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(spec.Executable) {
		return fmt.Errorf("executable must be an absolute path, got %q", spec.Executable)
	}
	if spec.LogDir != "" {
		if err := os.MkdirAll(spec.LogDir, 0o755); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}
	}
	return b.enable(spec)
}

// Disable removes the registration and stops a supervised daemon. Disabling
// when nothing is registered is a no-op.
func Disable() error {
	b, err := current()
	if err != nil {
		return err
	}
	return b.disable()
}

// Enabled reports whether the daemon is registered to start at login.
func Enabled() bool {
	if platform == nil {
		return false
	}
	ok, err := platform.enabled()
	return ok && err == nil
}

// Start starts the registered daemon now.
func Start() error { return withEnabled(backend.start) }

// Stop stops the registered daemon until the next login (or Start).
func Stop() error { return withEnabled(backend.stop) }

// Restart restarts the registered daemon from the binary currently on disk.
// On launchd this unloads and reloads the job, so a binary replaced by
// `easyeda update` is not refused against the old binary's cached signature.
func Restart() error { return withEnabled(backend.restart) }

func withEnabled(op func(backend) error) error {
	b, err := current()
	if err != nil {
		return err
	}
	if ok, err := b.enabled(); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("daemon autostart is not enabled; run `easyeda daemon autostart enable`")
	}
	return op(b)
}

// Query reports the registration state. It never fails: an unsupported
// platform or a query error is described in the result.
func Query(logDir string) Status {
	if platform == nil {
		return Status{Manager: unsupportedName(), Reason: ErrUnsupported.Error()}
	}
	st := Status{Supported: true, Manager: platform.manager(), Supervised: platform.supervised()}
	ok, err := platform.enabled()
	if err != nil {
		st.Reason = err.Error()
		return st
	}
	if !ok {
		return st
	}
	st.Enabled = true
	st.Logs = platform.logHint(logDir)
	if st.Running, err = platform.running(); err != nil {
		st.Reason = err.Error()
	}
	return st
}

// CapturedEnv returns the PassEnv variables set in this process.
func CapturedEnv() map[string]string {
	env := map[string]string{}
	for _, k := range PassEnv {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env[k] = v
		}
	}
	return env
}

// LogFiles are the stdout/stderr files of a daemon not logging to a journal.
func LogFiles(logDir string) (stdout, stderr string) {
	return filepath.Join(logDir, Name+".out.log"), filepath.Join(logDir, Name+".err.log")
}
