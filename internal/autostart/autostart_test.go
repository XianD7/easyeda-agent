package autostart

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCapturedEnvCopiesOnlyPathVariables(t *testing.T) {
	t.Setenv("CODEX_HOME", "/tmp/codex")
	t.Setenv("EASYEDA_AUDIT_DIR", "")
	t.Setenv("GITHUB_TOKEN", "secret")
	env := CapturedEnv()
	if env["CODEX_HOME"] != "/tmp/codex" {
		t.Fatalf("CODEX_HOME not captured: %v", env)
	}
	if _, ok := env["EASYEDA_AUDIT_DIR"]; ok {
		t.Fatal("empty variable captured")
	}
	if _, ok := env["GITHUB_TOKEN"]; ok {
		t.Fatal("credential captured into the service definition")
	}
}

func TestServiceConfigMarksManagedDaemon(t *testing.T) {
	c := serviceConfig(Spec{Executable: "/bin/easyeda", Arguments: []string{"daemon", "start"}, Env: map[string]string{"CODEX_HOME": "/c"}})
	if c.EnvVars[ManagedEnv] != "1" || c.EnvVars["CODEX_HOME"] != "/c" {
		t.Fatalf("env = %v", c.EnvVars)
	}
	if c.Name != Name || c.Option["UserService"] != true {
		t.Fatalf("config = %+v", c)
	}
	// Restart only on failure: a clean exit means another easyeda daemon took the port.
	if !strings.Contains(c.Option["SystemdScript"].(string), "Restart=on-failure") ||
		!strings.Contains(c.Option["SystemdScript"].(string), "WantedBy=default.target") ||
		!strings.Contains(c.Option["LaunchdConfig"].(string), "<key>SuccessfulExit</key>") {
		t.Fatal("unit templates must restart only on failure and start with the user session")
	}
}

func TestInteractive(t *testing.T) {
	t.Setenv(ManagedEnv, "")
	if !Interactive() {
		t.Fatal("terminal process reported as managed")
	}
	t.Setenv(ManagedEnv, "1")
	if Interactive() {
		t.Fatal("managed daemon reported as interactive")
	}
}

// TestAutostartLive registers a real login service on this machine (launchd,
// systemd --user or HKCU Run), swaps the binary the way `easyeda update` does,
// restarts through the manager and checks the new version serves. It only
// runs with EASYEDA_TEST_AUTOSTART=1 (CI sets it per OS) and never touches an
// existing registration.
func TestAutostartLive(t *testing.T) {
	if os.Getenv("EASYEDA_TEST_AUTOSTART") != "1" {
		t.Skip("set EASYEDA_TEST_AUTOSTART=1 to register a real login service")
	}
	if !Supported() {
		t.Skipf("unsupported platform: %s", unsupportedName())
	}
	if Enabled() {
		t.Skip("an autostart registration already exists; not replacing it")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "easyeda")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := func(out, ver string) {
		t.Helper()
		cmd := exec.Command("go", "build", "-o", out,
			"-ldflags", "-X github.com/zhoushoujianwork/easyeda-agent/internal/version.Version="+ver, "../../cmd/easyeda")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", ver, err, b)
		}
	}
	build(exe, "v0.0.0-autostart.1")
	port := freePort(t)
	ports := fmt.Sprintf("%d-%d", port, port)
	run := func(bin string, args ...string) string {
		t.Helper()
		out, err := exec.Command(bin, append(args, "--ports", ports)...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", filepath.Base(bin), args, err, out)
		}
		return string(out)
	}
	t.Cleanup(func() {
		_ = exec.Command(exe, "daemon", "autostart", "disable", "--ports", ports).Run()
		_ = Disable()
	})

	t.Log(run(exe, "daemon", "autostart", "enable", "--auto-update-skill=false"))
	waitVersion(t, port, "v0.0.0-autostart.1")
	var st struct {
		Enabled bool `json:"enabled"`
		Serving bool `json:"serving"`
	}
	if err := json.Unmarshal([]byte(run(exe, "daemon", "autostart", "status", "--json")), &st); err != nil || !st.Enabled || !st.Serving {
		t.Fatalf("status = %+v, %v", st, err)
	}

	// Replace the binary as `easyeda update` does (a running .exe can only be
	// renamed on Windows), then restart from the old binary's process.
	old := exe + ".old"
	if err := os.Rename(exe, old); err != nil {
		t.Fatal(err)
	}
	build(exe, "v0.0.0-autostart.2")
	if err := Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	waitVersion(t, port, "v0.0.0-autostart.2")

	t.Log(run(exe, "daemon", "autostart", "disable"))
	if Enabled() {
		t.Fatal("still registered after disable")
	}
	deadline := time.Now().Add(10 * time.Second)
	for health(port) != "" {
		if time.Now().After(deadline) {
			t.Fatal("daemon still serving after disable")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func health(port int) string {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var h struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&h) != nil {
		return ""
	}
	return h.Version
}

func waitVersion(t *testing.T, port int, want string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	got := ""
	for time.Now().Before(deadline) {
		if got = health(port); got == want {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("daemon on :%d serves %q, want %q", port, got, want)
}
