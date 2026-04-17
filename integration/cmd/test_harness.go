package clitest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	itest "github.com/GizClaw/gizclaw-go/integration/testutil"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/goccy/go-yaml"
)

const (
	fixtureListenAddrToken = "__LISTEN_ADDR__"
	serverStopTimeout      = 5 * time.Second
)

var (
	buildBinaryOnce sync.Once
	buildBinaryPath string
	buildBinaryErr  error
)

type Harness struct {
	t testing.TB

	RepoRoot string
	StoryDir string

	SandboxDir      string
	HomeDir         string
	XDGConfigHome   string
	ServerWorkspace string
	LogsDir         string

	BinaryPath      string
	ServerAddr      string
	ServerPublicKey string
	ServerLogPath   string

	lastFixtureName string
	serverRuns      int
	serverCmd       *exec.Cmd
	serverLog       *os.File
	serverWaitCh    chan error
}

type Result struct {
	Args   []string
	Stdout string
	Stderr string
	Err    error
}

type cliContextConfig struct {
	Server struct {
		Address   string `yaml:"address"`
		PublicKey string `yaml:"public-key"`
	} `yaml:"server"`
}

func (r Result) MustSucceed(t testing.TB) {
	t.Helper()
	if r.Err == nil {
		return
	}
	t.Fatalf("command %q failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(r.Args, " "), r.Err, r.Stdout, r.Stderr)
}

func NewHarness(t testing.TB, story string) *Harness {
	t.Helper()

	repoRoot := mustRepoRoot(t)
	sandboxDir := t.TempDir()
	homeDir := filepath.Join(sandboxDir, "home")
	xdgConfigHome := filepath.Join(sandboxDir, "xdg-config")
	serverWorkspace := filepath.Join(sandboxDir, "server-workspace")
	logsDir := filepath.Join(sandboxDir, "logs")
	for _, dir := range []string{homeDir, xdgConfigHome, serverWorkspace, logsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
	}

	h := &Harness{
		t:               t,
		RepoRoot:        repoRoot,
		StoryDir:        filepath.Join(repoRoot, "integration", "cmd", story),
		SandboxDir:      sandboxDir,
		HomeDir:         homeDir,
		XDGConfigHome:   xdgConfigHome,
		ServerWorkspace: serverWorkspace,
		LogsDir:         logsDir,
		BinaryPath:      mustBuildCLI(t, repoRoot),
	}
	t.Cleanup(func() { h.stopServer() })
	return h
}

func (h *Harness) StartServerFromFixture(fixtureName string) {
	h.t.Helper()

	if h.ServerAddr == "" {
		h.ServerAddr = itest.AllocateUDPAddr(h.t)
	}
	h.lastFixtureName = fixtureName
	h.PrepareServerWorkspaceFromFixture(fixtureName)
	h.startServerProcess()
}

func (h *Harness) PrepareServerWorkspaceFromFixture(fixtureName string) {
	h.t.Helper()

	h.lastFixtureName = fixtureName
	h.renderServerFixture(fixtureName, map[string]string{
		fixtureListenAddrToken: h.ServerAddr,
	})
}

func (h *Harness) RestartServer() {
	h.t.Helper()

	h.StopServer()
	h.startServerProcess()
}

func (h *Harness) StopServer() {
	h.t.Helper()
	h.stopServer()
}

func (h *Harness) startServerProcess() {
	h.t.Helper()

	h.serverRuns++
	logPath := filepath.Join(h.LogsDir, fmt.Sprintf("server-%02d.log", h.serverRuns))
	logFile, err := os.Create(logPath)
	if err != nil {
		h.t.Fatalf("create server log: %v", err)
	}

	cmd := exec.Command(h.BinaryPath, "serve", h.ServerWorkspace)
	cmd.Dir = h.RepoRoot
	cmd.Env = h.baseEnv()
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		h.t.Fatalf("start server: %v", err)
	}

	h.serverCmd = cmd
	h.serverLog = logFile
	h.ServerLogPath = logPath
	h.serverWaitCh = make(chan error, 1)
	go func() {
		h.serverWaitCh <- cmd.Wait()
		close(h.serverWaitCh)
	}()

	h.waitForServerIdentity()
	h.waitForServerReady()
}

func (h *Harness) CreateContext(name string) Result {
	h.t.Helper()
	return h.CreateContextWith(name, h.ServerAddr, h.ServerPublicKey)
}

func (h *Harness) CreateContextWith(name, serverAddr, serverPublicKey string) Result {
	h.t.Helper()
	return h.RunCLI(
		"context", "create", name,
		"--server", serverAddr,
		"--pubkey", serverPublicKey,
	)
}

func (h *Harness) RegisterContext(name, token string, extraArgs ...string) Result {
	h.t.Helper()

	args := []string{"play", "register", "--context", name}
	if token != "" {
		args = append(args, "--token", token)
	}
	args = append(args, extraArgs...)
	return h.RunCLI(args...)
}

func (h *Harness) WaitForPing(contextName string) {
	h.t.Helper()

	if _, err := h.RunCLIUntilSuccess("ping", "--context", contextName); err != nil {
		h.t.Fatalf("context %q did not become ping-ready: %v", contextName, err)
	}
}

func (h *Harness) UseContext(name string) Result {
	h.t.Helper()
	return h.RunCLI("context", "use", name)
}

func (h *Harness) ListContexts() Result {
	h.t.Helper()
	return h.RunCLI("context", "list")
}

func (h *Harness) ContextPublicKey(name string) string {
	h.t.Helper()

	keyPair, err := loadIdentity(filepath.Join(h.contextRoot(), name, "identity.key"))
	if err != nil {
		h.t.Fatalf("load context %q identity: %v", name, err)
	}
	return keyPair.Public.String()
}

func (h *Harness) ConnectClientFromContext(name string) *gizclaw.Client {
	h.t.Helper()

	client, err := h.connectClientFromContext(name)
	if err != nil {
		h.t.Fatalf("connect client from context %q: %v", name, err)
	}
	return client
}

func (h *Harness) RunCLI(args ...string) Result {
	h.t.Helper()

	cmd := exec.Command(h.BinaryPath, args...)
	cmd.Dir = h.SandboxDir
	cmd.Env = h.baseEnv()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return Result{
		Args:   append([]string(nil), args...),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    err,
	}
}

func (h *Harness) connectClientFromContext(name string) (*gizclaw.Client, error) {
	contextDir := filepath.Join(h.contextRoot(), name)
	data, err := os.ReadFile(filepath.Join(contextDir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read context config: %w", err)
	}

	var cfg cliContextConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse context config: %w", err)
	}

	keyPair, err := loadIdentity(filepath.Join(contextDir, "identity.key"))
	if err != nil {
		return nil, fmt.Errorf("load context identity: %w", err)
	}

	serverPublicKey, err := giznet.KeyFromHex(cfg.Server.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse server public key: %w", err)
	}

	client := &gizclaw.Client{KeyPair: keyPair}
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.DialAndServe(serverPublicKey, cfg.Server.Address)
	}()

	deadline := time.Now().Add(itest.ReadyTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			if err != nil {
				_ = client.Close()
				return nil, err
			}
			_ = client.Close()
			return nil, fmt.Errorf("client stopped before ready")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), itest.ProbeTimeout)
		err := probeServerPublicReady(ctx, client)
		cancel()
		if err == nil {
			return client, nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	_ = client.Close()
	return nil, fmt.Errorf("timeout waiting for client readiness")
}

func (h *Harness) RunCLIUntilSuccess(args ...string) (Result, error) {
	var last Result
	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		last = h.RunCLI(args...)
		if last.Err != nil {
			return fmt.Errorf("command %q failed: %w\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), last.Err, last.Stdout, last.Stderr)
		}
		return nil
	}); err != nil {
		return last, fmt.Errorf("command %q did not succeed before timeout: %w", strings.Join(args, " "), err)
	}
	return last, nil
}

func (h *Harness) waitForServerIdentity() {
	h.t.Helper()

	identityPath := filepath.Join(h.ServerWorkspace, "identity.key")
	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		if err := h.serverProcessError(); err != nil {
			return err
		}
		keyPair, err := loadIdentity(identityPath)
		if err != nil {
			return err
		}
		h.ServerPublicKey = keyPair.Public.String()
		return nil
	}); err != nil {
		h.t.Fatalf("server identity not ready: %v\nserver log: %s", err, h.ServerLogPath)
	}
}

func (h *Harness) waitForServerReady() {
	h.t.Helper()

	serverPublicKey, err := giznet.KeyFromHex(h.ServerPublicKey)
	if err != nil {
		h.t.Fatalf("parse server public key: %v", err)
	}

	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		if err := h.serverProcessError(); err != nil {
			return err
		}

		keyPair, err := giznet.GenerateKeyPair()
		if err != nil {
			return err
		}
		client := &gizclaw.Client{KeyPair: keyPair}
		errCh := make(chan error, 1)
		go func() {
			errCh <- client.DialAndServe(serverPublicKey, h.ServerAddr)
		}()
		defer client.Close()

		for i := 0; i < 20; i++ {
			select {
			case err := <-errCh:
				if err != nil {
					return err
				}
				return fmt.Errorf("client stopped before ready")
			default:
			}

			ctx, cancel := context.WithTimeout(context.Background(), itest.ProbeTimeout)
			err = probeServerPublicReady(ctx, client)
			cancel()
			if err == nil {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("server public probe did not become ready")
	}); err != nil {
		h.t.Fatalf("server did not become ready: %v\nserver log: %s", err, h.ServerLogPath)
	}
}

func (h *Harness) renderServerFixture(fixtureName string, replacements map[string]string) {
	h.t.Helper()

	fixturePath := filepath.Join(h.StoryDir, fixtureName)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		h.t.Fatalf("read fixture %q: %v", fixturePath, err)
	}

	rendered := string(data)
	for old, newValue := range replacements {
		rendered = strings.ReplaceAll(rendered, old, newValue)
	}

	targetPath := filepath.Join(h.ServerWorkspace, "config.yaml")
	if err := os.WriteFile(targetPath, []byte(rendered), 0o644); err != nil {
		h.t.Fatalf("write rendered config %q: %v", targetPath, err)
	}
}

func (h *Harness) baseEnv() []string {
	env := os.Environ()
	env = append(env,
		"HOME="+h.HomeDir,
		"XDG_CONFIG_HOME="+h.XDGConfigHome,
	)
	return env
}

func (h *Harness) contextRoot() string {
	return filepath.Join(h.XDGConfigHome, "gizclaw")
}

func (h *Harness) serverProcessError() error {
	if h.serverWaitCh == nil {
		return nil
	}
	select {
	case err, ok := <-h.serverWaitCh:
		h.serverWaitCh = nil
		if !ok {
			return fmt.Errorf("server exited before readiness")
		}
		if err != nil {
			return fmt.Errorf("server exited early: %w", err)
		}
		return fmt.Errorf("server exited before readiness")
	default:
		return nil
	}
}

func (h *Harness) stopServer() {
	if h.serverCmd == nil {
		return
	}

	defer func() {
		if failed, ok := h.t.(interface{ Failed() bool }); ok && failed.Failed() && h.ServerLogPath != "" {
			if data, err := os.ReadFile(h.ServerLogPath); err == nil && len(data) > 0 {
				h.t.Logf("CLI integration server log contents:\n%s", string(data))
			}
		}
		if h.serverLog != nil {
			_ = h.serverLog.Close()
		}
		if failed, ok := h.t.(interface{ Failed() bool }); ok && failed.Failed() {
			h.t.Logf("CLI integration server log: %s", h.ServerLogPath)
		}
		h.serverCmd = nil
		h.serverLog = nil
		h.serverWaitCh = nil
	}()

	if h.serverCmd.Process == nil {
		return
	}
	if h.serverCmd.ProcessState != nil && h.serverCmd.ProcessState.Exited() {
		return
	}

	_ = h.serverCmd.Process.Signal(os.Interrupt)

	if h.serverWaitCh != nil {
		select {
		case <-h.serverWaitCh:
		case <-time.After(serverStopTimeout):
			_ = h.serverCmd.Process.Kill()
			<-h.serverWaitCh
		}
	}
}

func mustRepoRoot(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration/cmd harness path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func mustBuildCLI(t testing.TB, repoRoot string) string {
	t.Helper()

	buildBinaryOnce.Do(func() {
		outDir, err := os.MkdirTemp("", "gizclaw-cli-bin-*")
		if err != nil {
			buildBinaryErr = err
			return
		}

		binaryPath := filepath.Join(outDir, "gizclaw")
		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gizclaw")
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildBinaryErr = fmt.Errorf("build gizclaw CLI: %w\n%s", err, string(output))
			return
		}
		buildBinaryPath = binaryPath
	})

	if buildBinaryErr != nil {
		t.Fatalf("build CLI binary: %v", buildBinaryErr)
	}
	return buildBinaryPath
}

func loadIdentity(path string) (*giznet.KeyPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) != giznet.KeySize {
		return nil, fmt.Errorf("invalid key file: got %d bytes, want %d", len(data), giznet.KeySize)
	}
	var key giznet.Key
	copy(key[:], data)
	return giznet.NewKeyPair(key)
}

func probeServerPublicReady(ctx context.Context, client *gizclaw.Client) error {
	api, err := client.ServerPublicClient()
	if err != nil {
		return err
	}
	resp, err := api.GetServerInfoWithResponse(ctx)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		if resp.StatusCode() != 0 {
			return fmt.Errorf("unexpected server info status %d", resp.StatusCode())
		}
		return fmt.Errorf("missing server info response body")
	}
	var _ serverpublic.ServerInfo = *resp.JSON200
	return nil
}
