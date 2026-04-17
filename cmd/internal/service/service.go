package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	kservice "github.com/kardianos/service"
)

type Spec struct {
	WorkspaceRoot string
	Executable    string
	Label         string
}

type Marker struct {
	Managed       bool   `json:"managed"`
	ServiceName   string `json:"service_name"`
	ServiceType   string `json:"service_type"`
	WorkspaceRoot string `json:"workspace"`
}

type StatusInfo struct {
	Installed     bool
	Running       bool
	ServiceName   string
	State         string
	WorkspaceRoot string
}

type installRecord struct {
	WorkspaceRoot string `json:"workspace"`
}

type managedService interface {
	Run() error
	Start() error
	Stop() error
	Restart() error
	Install() error
	Uninstall() error
	Status() (kservice.Status, error)
}

const serviceLabel = "com.gizclaw.serve"
const serviceDisplayName = "GizClaw Server"
const serviceDescription = "GizClaw server bound to a fixed workspace"
const serviceType = "system-service"
const workspaceServiceFile = "service.json"
const installedServiceFile = "installed-service.json"
const InternalRunFlag = "--service-run-workspace"

var userConfigDir = os.UserConfigDir
var executablePath = os.Executable
var newSystemService = func(spec Spec, program kservice.Interface) (managedService, error) {
	svc, err := kservice.New(program, newServiceConfig(spec))
	if err != nil {
		return nil, err
	}
	return svc, nil
}

type noopProgram struct{}

func (noopProgram) Start(kservice.Service) error { return nil }
func (noopProgram) Stop(kservice.Service) error  { return nil }

func resolveWorkspaceRoot(workspace string) (string, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("service: resolve workspace %q: %w", workspace, err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("service: create workspace %q: %w", root, err)
	}
	return root, nil
}

func serviceSpec(workspace string) (Spec, error) {
	root, err := resolveWorkspaceRoot(workspace)
	if err != nil {
		return Spec{}, err
	}
	exe, err := executablePath()
	if err != nil {
		return Spec{}, fmt.Errorf("service: resolve executable: %w", err)
	}
	return Spec{
		WorkspaceRoot: root,
		Executable:    exe,
		Label:         serviceLabel,
	}, nil
}

func NewService(workspace string, program kservice.Interface) (managedService, error) {
	spec, err := serviceSpec(workspace)
	if err != nil {
		return nil, err
	}
	svc, err := newSystemService(spec, program)
	if err != nil {
		return nil, fmt.Errorf("service: create service: %w", err)
	}
	return svc, nil
}

func Install(workspace string) error {
	spec, err := serviceSpec(workspace)
	if err != nil {
		return err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return fmt.Errorf("service: create service: %w", err)
	}
	if installed, err := serviceInstalled(svc); err != nil {
		return err
	} else if installed {
		return fmt.Errorf("service: already installed; run 'gizclaw service uninstall' first")
	}
	if _, err := readInstallRecord(); err == nil {
		return fmt.Errorf("service: already installed; run 'gizclaw service uninstall' first")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("service: read install record: %w", err)
	}
	if err := svc.Install(); err != nil {
		return fmt.Errorf("service: install: %w", err)
	}
	if err := WriteMarker(spec.WorkspaceRoot); err != nil {
		_ = svc.Uninstall()
		return err
	}
	if err := writeInstallRecord(spec); err != nil {
		_ = RemoveMarker(spec.WorkspaceRoot)
		_ = svc.Uninstall()
		return err
	}
	return nil
}

func Start() error {
	spec, err := installedServiceSpec()
	if err != nil {
		return err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return fmt.Errorf("service: create service: %w", err)
	}
	if installed, err := serviceInstalled(svc); err != nil {
		return err
	} else if !installed {
		return fmt.Errorf("service: not installed")
	}
	return svc.Start()
}

func Restart() error {
	spec, err := installedServiceSpec()
	if err != nil {
		return err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return fmt.Errorf("service: create service: %w", err)
	}
	if installed, err := serviceInstalled(svc); err != nil {
		return err
	} else if !installed {
		return fmt.Errorf("service: not installed")
	}
	return svc.Restart()
}

func Status() (StatusInfo, error) {
	spec, err := statusSpec()
	if err != nil {
		return StatusInfo{}, err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return StatusInfo{}, fmt.Errorf("service: create service: %w", err)
	}
	status, err := svc.Status()
	if err != nil {
		if errors.Is(err, kservice.ErrNotInstalled) {
			return StatusInfo{
				Installed:     false,
				Running:       false,
				ServiceName:   serviceLabel,
				State:         "not installed",
				WorkspaceRoot: spec.WorkspaceRoot,
			}, nil
		}
		return StatusInfo{}, fmt.Errorf("service: status: %w", err)
	}
	return StatusInfo{
		Installed:     true,
		Running:       status == kservice.StatusRunning,
		ServiceName:   serviceLabel,
		State:         serviceStateString(status),
		WorkspaceRoot: spec.WorkspaceRoot,
	}, nil
}

func Stop() error {
	spec, err := installedServiceSpec()
	if err != nil {
		return err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return fmt.Errorf("service: create service: %w", err)
	}
	status, err := svc.Status()
	if err != nil {
		if errors.Is(err, kservice.ErrNotInstalled) {
			return fmt.Errorf("service: not installed")
		}
		return fmt.Errorf("service: status: %w", err)
	}
	if status == kservice.StatusStopped {
		return nil
	}
	return svc.Stop()
}

func Uninstall() error {
	spec, err := installedServiceSpec()
	if err != nil {
		return err
	}
	svc, err := newSystemService(spec, noopProgram{})
	if err != nil {
		return fmt.Errorf("service: create service: %w", err)
	}
	status, err := svc.Status()
	if err != nil {
		if errors.Is(err, kservice.ErrNotInstalled) {
			return fmt.Errorf("service: not installed")
		}
		return fmt.Errorf("service: status: %w", err)
	}
	if status == kservice.StatusRunning {
		if err := svc.Stop(); err != nil {
			return fmt.Errorf("service: stop before uninstall: %w", err)
		}
	}
	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("service: uninstall: %w", err)
	}
	if spec.WorkspaceRoot != "" {
		if err := RemoveMarker(spec.WorkspaceRoot); err != nil {
			return err
		}
	}
	if err := removeInstallRecord(); err != nil {
		return err
	}
	return nil
}

func newServiceConfig(spec Spec) *kservice.Config {
	return &kservice.Config{
		Name:             spec.Label,
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
		Executable:       spec.Executable,
		Arguments:        []string{InternalRunFlag, spec.WorkspaceRoot},
		WorkingDirectory: spec.WorkspaceRoot,
		Option: kservice.KeyValue{
			"KeepAlive":  true,
			"RunAtLoad":  false,
			"UserService": true,
		},
	}
}

func serviceInstalled(svc managedService) (bool, error) {
	_, err := svc.Status()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, kservice.ErrNotInstalled) {
		return false, nil
	}
	return false, fmt.Errorf("service: status: %w", err)
}

func statusSpec() (Spec, error) {
	spec := Spec{Label: serviceLabel}
	if exe, err := executablePath(); err == nil {
		spec.Executable = exe
	}
	record, err := readInstallRecord()
	if err == nil {
		spec.WorkspaceRoot = record.WorkspaceRoot
		return spec, nil
	}
	if os.IsNotExist(err) {
		return spec, nil
	}
	return Spec{}, fmt.Errorf("service: read install record: %w", err)
}

func installedServiceSpec() (Spec, error) {
	record, err := readInstallRecord()
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, fmt.Errorf("service: not installed")
		}
		return Spec{}, fmt.Errorf("service: read install record: %w", err)
	}
	return serviceSpec(record.WorkspaceRoot)
}

func serviceStateString(status kservice.Status) string {
	switch status {
	case kservice.StatusRunning:
		return "running"
	case kservice.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func markerPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, workspaceServiceFile)
}

func installRecordPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("service: resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "gizclaw", installedServiceFile), nil
}

func writeInstallRecord(spec Spec) error {
	path, err := installRecordPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("service: create install record dir: %w", err)
	}
	data, err := json.MarshalIndent(installRecord{WorkspaceRoot: spec.WorkspaceRoot}, "", "  ")
	if err != nil {
		return fmt.Errorf("service: marshal install record: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("service: write install record: %w", err)
	}
	return nil
}

func readInstallRecord() (installRecord, error) {
	path, err := installRecordPath()
	if err != nil {
		return installRecord{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return installRecord{}, err
	}
	var record installRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return installRecord{}, fmt.Errorf("service: parse install record: %w", err)
	}
	if record.WorkspaceRoot == "" {
		return installRecord{}, fmt.Errorf("service: install record missing workspace")
	}
	return record, nil
}

func removeInstallRecord() error {
	path, err := installRecordPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: remove install record: %w", err)
	}
	return nil
}

func WriteMarker(workspace string) error {
	root, err := resolveWorkspaceRoot(workspace)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(Marker{
		Managed:       true,
		ServiceName:   serviceLabel,
		ServiceType:   serviceType,
		WorkspaceRoot: root,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("service: marshal marker: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(markerPath(root), data, 0o644); err != nil {
		return fmt.Errorf("service: write marker: %w", err)
	}
	return nil
}

func RemoveMarker(workspace string) error {
	root, err := resolveWorkspaceRoot(workspace)
	if err != nil {
		return err
	}
	if err := os.Remove(markerPath(root)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: remove marker: %w", err)
	}
	return nil
}

func WorkspaceManaged(workspace string) (bool, error) {
	root, err := resolveWorkspaceRoot(workspace)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(markerPath(root))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("service: read marker: %w", err)
	}
	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return false, fmt.Errorf("service: parse marker: %w", err)
	}
	return marker.Managed, nil
}
