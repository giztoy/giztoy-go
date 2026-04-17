package service

import (
	"errors"
	"testing"
	"os"

	kservice "github.com/kardianos/service"
)

func TestServiceLabelConstant(t *testing.T) {
	if serviceLabel != "com.gizclaw.serve" {
		t.Fatalf("serviceLabel = %q", serviceLabel)
	}
}

func TestNewServiceConfigUsesInternalRunCommand(t *testing.T) {
	spec := Spec{
		WorkspaceRoot: "/tmp/workspace",
		Executable:    "/usr/local/bin/gizclaw",
		Label:         "com.gizclaw.serve.test",
	}
	cfg := newServiceConfig(spec)
	if cfg.Name != spec.Label {
		t.Fatalf("cfg.Name = %q", cfg.Name)
	}
	if cfg.Executable != spec.Executable {
		t.Fatalf("cfg.Executable = %q", cfg.Executable)
	}
	if cfg.WorkingDirectory != spec.WorkspaceRoot {
		t.Fatalf("cfg.WorkingDirectory = %q", cfg.WorkingDirectory)
	}
	for _, want := range []string{
		InternalRunFlag,
		spec.WorkspaceRoot,
	} {
		if !contains(cfg.Arguments, want) {
			t.Fatalf("cfg.Arguments = %#v, missing %q", cfg.Arguments, want)
		}
	}
	if keepAlive, ok := cfg.Option["KeepAlive"].(bool); !ok || !keepAlive {
		t.Fatalf("cfg.Option[KeepAlive] = %#v", cfg.Option["KeepAlive"])
	}
	if runAtLoad, ok := cfg.Option["RunAtLoad"].(bool); !ok || runAtLoad {
		t.Fatalf("cfg.Option[RunAtLoad] = %#v", cfg.Option["RunAtLoad"])
	}
	if userService, ok := cfg.Option["UserService"].(bool); !ok || !userService {
		t.Fatalf("cfg.Option[UserService] = %#v", cfg.Option["UserService"])
	}
}

func TestRuntimeWorkspaceFromArgs(t *testing.T) {
	workspace, ok, err := RuntimeWorkspaceFromArgs([]string{"--other", "value", InternalRunFlag, "/tmp/workspace"})
	if err != nil {
		t.Fatalf("RuntimeWorkspaceFromArgs() error = %v", err)
	}
	if !ok {
		t.Fatal("RuntimeWorkspaceFromArgs() should detect internal run flag")
	}
	if workspace != "/tmp/workspace" {
		t.Fatalf("RuntimeWorkspaceFromArgs() workspace = %q", workspace)
	}
}

func TestRuntimeWorkspaceFromArgsMissingValue(t *testing.T) {
	if _, ok, err := RuntimeWorkspaceFromArgs([]string{InternalRunFlag}); err == nil || ok {
		t.Fatalf("RuntimeWorkspaceFromArgs() = (_, %t, %v), want error", ok, err)
	}
}

func TestInstallWritesMarkerAndRecord(t *testing.T) {
	restore := stubPaths(t)
	defer restore()

	workspace := t.TempDir()
	fake := &fakeManagedService{statusErr: kservice.ErrNotInstalled}
	newSystemService = func(spec Spec, program kservice.Interface) (managedService, error) {
		return fake, nil
	}
	if err := Install(workspace); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !fake.installCalled {
		t.Fatal("Install() should call managed service Install")
	}
	managed, err := WorkspaceManaged(workspace)
	if err != nil {
		t.Fatalf("WorkspaceManaged() error = %v", err)
	}
	if !managed {
		t.Fatal("workspace should be marked managed after install")
	}
	record, err := readInstallRecord()
	if err != nil {
		t.Fatalf("readInstallRecord() error = %v", err)
	}
	if record.WorkspaceRoot != workspace {
		t.Fatalf("record.WorkspaceRoot = %q", record.WorkspaceRoot)
	}
}

func TestStatusReturnsNotInstalledWithoutError(t *testing.T) {
	restore := stubPaths(t)
	defer restore()

	fake := &fakeManagedService{statusErr: kservice.ErrNotInstalled}
	newSystemService = func(spec Spec, program kservice.Interface) (managedService, error) {
		return fake, nil
	}

	info, err := Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Installed {
		t.Fatal("Status().Installed should be false")
	}
	if info.Running {
		t.Fatal("Status().Running should be false")
	}
	if info.State != "not installed" {
		t.Fatalf("Status().State = %q", info.State)
	}
	if info.ServiceName != serviceLabel {
		t.Fatalf("Status().ServiceName = %q", info.ServiceName)
	}
}

func TestStatusReturnsRunningInfo(t *testing.T) {
	restore := stubPaths(t)
	defer restore()

	workspace := t.TempDir()
	if err := writeInstallRecord(Spec{WorkspaceRoot: workspace}); err != nil {
		t.Fatalf("writeInstallRecord() error = %v", err)
	}

	fake := &fakeManagedService{status: kservice.StatusRunning}
	newSystemService = func(spec Spec, program kservice.Interface) (managedService, error) {
		return fake, nil
	}

	info, err := Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !info.Installed {
		t.Fatal("Status().Installed should be true")
	}
	if !info.Running {
		t.Fatal("Status().Running should be true")
	}
	if info.State != "running" {
		t.Fatalf("Status().State = %q", info.State)
	}
	if info.WorkspaceRoot != workspace {
		t.Fatalf("Status().WorkspaceRoot = %q", info.WorkspaceRoot)
	}
}

func TestUninstallStopsServiceAndRemovesState(t *testing.T) {
	restore := stubPaths(t)
	defer restore()

	workspace := t.TempDir()
	if err := WriteMarker(workspace); err != nil {
		t.Fatalf("WriteMarker() error = %v", err)
	}
	if err := writeInstallRecord(Spec{WorkspaceRoot: workspace}); err != nil {
		t.Fatalf("writeInstallRecord() error = %v", err)
	}

	fake := &fakeManagedService{status: kservice.StatusRunning}
	newSystemService = func(spec Spec, program kservice.Interface) (managedService, error) {
		return fake, nil
	}
	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !fake.stopCalled {
		t.Fatal("Uninstall() should stop a running service")
	}
	if !fake.uninstallCalled {
		t.Fatal("Uninstall() should uninstall the managed service")
	}
	managed, err := WorkspaceManaged(workspace)
	if err != nil {
		t.Fatalf("WorkspaceManaged() error = %v", err)
	}
	if managed {
		t.Fatal("workspace should not be managed after uninstall")
	}
	if _, err := readInstallRecord(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readInstallRecord() error = %v, want os.ErrNotExist", err)
	}
}

func TestWorkspaceManagedMarkerLifecycle(t *testing.T) {
	workspace := t.TempDir()
	managed, err := WorkspaceManaged(workspace)
	if err != nil {
		t.Fatalf("WorkspaceManaged(initial) error = %v", err)
	}
	if managed {
		t.Fatal("workspace should not be managed before marker write")
	}

	if err := WriteMarker(workspace); err != nil {
		t.Fatalf("WriteMarker error = %v", err)
	}
	managed, err = WorkspaceManaged(workspace)
	if err != nil {
		t.Fatalf("WorkspaceManaged(after write) error = %v", err)
	}
	if !managed {
		t.Fatal("workspace should be managed after marker write")
	}

	if err := RemoveMarker(workspace); err != nil {
		t.Fatalf("RemoveMarker error = %v", err)
	}
	managed, err = WorkspaceManaged(workspace)
	if err != nil {
		t.Fatalf("WorkspaceManaged(after remove) error = %v", err)
	}
	if managed {
		t.Fatal("workspace should not be managed after marker removal")
	}
}

type fakeManagedService struct {
	status         kservice.Status
	statusErr      error
	installCalled  bool
	restartCalled  bool
	stopCalled     bool
	uninstallCalled bool
}

func (f *fakeManagedService) Run() error { return nil }

func (f *fakeManagedService) Start() error { return nil }

func (f *fakeManagedService) Stop() error {
	f.stopCalled = true
	return nil
}

func (f *fakeManagedService) Restart() error {
	f.restartCalled = true
	return nil
}

func (f *fakeManagedService) Install() error {
	f.installCalled = true
	return nil
}

func (f *fakeManagedService) Uninstall() error {
	f.uninstallCalled = true
	return nil
}

func (f *fakeManagedService) Status() (kservice.Status, error) {
	return f.status, f.statusErr
}

func stubPaths(t *testing.T) func() {
	t.Helper()
	oldUserConfigDir := userConfigDir
	oldExecutablePath := executablePath
	oldNewSystemService := newSystemService
	configDir := t.TempDir()
	userConfigDir = func() (string, error) { return configDir, nil }
	executablePath = func() (string, error) { return "/usr/local/bin/gizclaw", nil }
	return func() {
		userConfigDir = oldUserConfigDir
		executablePath = oldExecutablePath
		newSystemService = oldNewSystemService
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
