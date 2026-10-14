package autostart

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kardianos/service"
)

const (
	platformLaunchd = "darwin-launchd"
	platformSystemd = "linux-systemd"
)

// Both units restart the daemon only when it fails. A clean exit is the daemon
// being stopped or replaced by another easyeda daemon on the same port (a
// terminal `daemon start`); restarting then would take the port straight back.

// launchdAgent follows the mutagen agent: load only in GUI login sessions
// (Aqua), not again for every SSH session.
const launchdAgent = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{Name | html}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{Path | html}}</string>
{{range Arguments}}		<string>{{. | html}}</string>
{{end}}	</array>
	{{if EnvVars}}<key>EnvironmentVariables</key>
	<dict>
{{range EnvVars}}{{.}}
{{end}}	</dict>
	{{end}}<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>LimitLoadToSessionType</key>
	<string>Aqua</string>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>{{StandardOutPath | html}}</string>
	<key>StandardErrorPath</key>
	<string>{{StandardErrorPath | html}}</string>
</dict>
</plist>
`

// systemdUserUnit follows the syncthing user unit. The library's default
// targets multi-user.target, which a user manager never reaches, so the unit
// would not start at login.
const systemdUserUnit = `[Unit]
Description={{Description}}
ConditionFileIsExecutable={{Path | cmdEscape}}
StartLimitIntervalSec=60
StartLimitBurst=4

[Service]
ExecStart={{Path | cmdEscape}}{{range Arguments}} {{. | cmd}}{{end}}
Restart=on-failure
RestartSec=2
{{range EnvVars}}{{.}}
{{end}}
[Install]
WantedBy=default.target
`

func init() {
	if p := service.Platform(); platform == nil && (p == platformLaunchd || p == platformSystemd) {
		platform = serviceBackend{platform: p}
	}
}

func unsupportedName() string { return service.Platform() }

// serviceBackend drives a per-user launchd agent or systemd --user unit.
type serviceBackend struct{ platform string }

func (b serviceBackend) manager() string {
	if b.platform == platformLaunchd {
		return "launchd"
	}
	return "systemd-user"
}

func (serviceBackend) supervised() bool { return true }

func (b serviceBackend) logHint(logDir string) string {
	if b.platform == platformSystemd {
		return "journalctl --user -u " + Name
	}
	out, errLog := LogFiles(logDir)
	return out + ", " + errLog
}

func serviceConfig(spec Spec) *service.Config {
	env := map[string]string{ManagedEnv: "1"}
	for k, v := range spec.Env {
		env[k] = v
	}
	return &service.Config{
		Name:        Name,
		DisplayName: "easyeda-agent daemon",
		Description: "easyeda-agent daemon for the EasyEDA connector",
		Executable:  spec.Executable,
		Arguments:   spec.Arguments,
		EnvVars:     env,
		Option: service.KeyValue{
			"UserService":   true,
			"LogDirectory":  spec.LogDir,
			"LaunchdConfig": launchdAgent,
			"SystemdScript": systemdUserUnit,
		},
	}
}

// newService is replaced in tests.
var newService = func(c *service.Config) (service.Service, error) { return service.New(noop{}, c) }

func (serviceBackend) open(spec Spec) (service.Service, error) {
	return newService(serviceConfig(spec))
}

func (b serviceBackend) enable(spec Spec) error {
	svc, err := b.open(spec)
	if err != nil {
		return err
	}
	if _, err := svc.Status(); !errors.Is(err, service.ErrNotInstalled) {
		_ = svc.Stop()
		if err := svc.Uninstall(); err != nil {
			return fmt.Errorf("remove previous registration: %w", err)
		}
	}
	if err := svc.Install(); err != nil {
		return fmt.Errorf("register %s: %w", Name, err)
	}
	if err := svc.Start(); err != nil {
		return fmt.Errorf("start %s: %w", Name, err)
	}
	return nil
}

func (b serviceBackend) disable() error {
	svc, err := b.open(Spec{})
	if err != nil {
		return err
	}
	if _, err := svc.Status(); errors.Is(err, service.ErrNotInstalled) {
		return nil
	}
	_ = svc.Stop() // systemd's Uninstall only disables; stop the running unit first
	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("remove %s: %w", Name, err)
	}
	return nil
}

func (b serviceBackend) state() (service.Status, error) {
	svc, err := b.open(Spec{})
	if err != nil {
		return service.StatusUnknown, err
	}
	return svc.Status()
}

func (b serviceBackend) enabled() (bool, error) {
	_, err := b.state()
	if errors.Is(err, service.ErrNotInstalled) {
		return false, nil
	}
	// Any other status error (e.g. a failed unit) still means it is registered.
	return true, nil
}

func (b serviceBackend) running() (*bool, error) {
	s, err := b.state()
	if err != nil {
		return nil, err
	}
	r := s == service.StatusRunning
	return &r, nil
}

func (b serviceBackend) start() error {
	svc, err := b.open(Spec{})
	if err != nil {
		return err
	}
	if s, err := svc.Status(); err == nil && s == service.StatusRunning {
		return nil
	}
	if b.platform == platformLaunchd {
		return reload(svc)
	}
	return svc.Start()
}

func (b serviceBackend) stop() error {
	svc, err := b.open(Spec{})
	if err != nil {
		return err
	}
	if s, err := svc.Status(); err == nil && s == service.StatusStopped {
		return nil
	}
	return svc.Stop()
}

func (b serviceBackend) restart() error {
	svc, err := b.open(Spec{})
	if err != nil {
		return err
	}
	if b.platform == platformLaunchd {
		return reload(svc)
	}
	return svc.Restart()
}

// reload unloads and loads the launchd job. A job that exited cleanly stays
// loaded but idle, and `launchctl load` then fails with "Load failed: 5:
// Input/output error"; unloading first (ignoring "not loaded") always gives a
// fresh process, from the binary currently on disk.
func reload(svc service.Service) error {
	_ = svc.Stop()
	return svc.Start()
}

// unitPath is where the library writes the registration for this platform.
func (b serviceBackend) unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if b.platform == platformLaunchd {
		return filepath.Join(home, "Library", "LaunchAgents", Name+".plist"), nil
	}
	return filepath.Join(home, ".config", "systemd", "user", Name+".service"), nil
}

func (b serviceBackend) executable() (string, error) {
	p, err := b.unitPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if b.platform == platformLaunchd {
		return plistExecutable(string(data))
	}
	return unitExecutable(string(data))
}

var plistProgram = regexp.MustCompile(`<key>ProgramArguments</key>\s*<array>\s*<string>([^<]*)</string>`)

func plistExecutable(plist string) (string, error) {
	m := plistProgram.FindStringSubmatch(plist)
	if m == nil {
		return "", errors.New("no ProgramArguments in the launchd agent")
	}
	return html.UnescapeString(m[1]), nil
}

// unitExecutable reverses the library's cmdEscape (spaces as \x20).
func unitExecutable(unit string) (string, error) {
	for _, line := range strings.Split(unit, "\n") {
		if rest, ok := strings.CutPrefix(line, "ExecStart="); ok {
			path, _, _ := strings.Cut(rest, " ")
			return strings.ReplaceAll(path, `\x20`, " "), nil
		}
	}
	return "", errors.New("no ExecStart in the systemd unit")
}

// noop satisfies service.Interface; the daemon runs through its own
// `daemon start` command, never through service.Run.
type noop struct{}

func (noop) Start(service.Service) error { return nil }
func (noop) Stop(service.Service) error  { return nil }
