package firmware

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"testing"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
)

type testEnv struct {
	t     *testing.T
	root  string
	store depotstore.Store
	srv   *Server
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	root := t.TempDir()
	store := depotstore.Dir(root)
	return &testEnv{
		t:     t,
		root:  root,
		store: store,
		srv:   &Server{Store: store},
	}
}

func (e *testEnv) writeFile(name, content string) {
	e.t.Helper()
	if err := e.store.WriteFile(name, []byte(content)); err != nil {
		e.t.Fatalf("write %s: %v", name, err)
	}
}

func (e *testEnv) writeJSON(name string, value any) {
	e.t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		e.t.Fatalf("marshal %s: %v", name, err)
	}
	data = append(data, '\n')
	if err := e.store.WriteFile(name, data); err != nil {
		e.t.Fatalf("write %s: %v", name, err)
	}
}

func (e *testEnv) readFile(name string) []byte {
	e.t.Helper()
	data, err := e.store.ReadFile(name)
	if err != nil {
		e.t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func (e *testEnv) writeDepotInfo(depot string, paths ...string) apitypes.DepotInfo {
	e.t.Helper()
	info := depotInfo(paths...)
	e.writeJSON(path.Join(depot, "info.json"), info)
	return info
}

func (e *testEnv) writeRelease(depot string, channel Channel, version string, files map[string]string) apitypes.DepotRelease {
	e.t.Helper()
	release := depotReleaseForFiles(channel, version, files)
	for filePath, content := range files {
		e.writeFile(path.Join(depot, string(channel), filePath), content)
	}
	e.writeJSON(path.Join(depot, string(channel), "manifest.json"), release)
	return release
}

func depotInfo(paths ...string) apitypes.DepotInfo {
	files := make([]apitypes.DepotInfoFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, apitypes.DepotInfoFile{Path: p})
	}
	return apitypes.DepotInfo{Files: &files}
}

func depotReleaseForFiles(channel Channel, version string, files map[string]string) apitypes.DepotRelease {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]apitypes.DepotFile, 0, len(paths))
	for _, p := range paths {
		md5Hex, shaHex := fileDigests([]byte(files[p]))
		out = append(out, apitypes.DepotFile{
			Md5:    md5Hex,
			Path:   p,
			Sha256: shaHex,
		})
	}
	return apitypes.DepotRelease{
		Channel:        stringPtr(string(channel)),
		Files:          &out,
		FirmwareSemver: version,
	}
}

func fileDigests(data []byte) (string, string) {
	md5Sum := md5.Sum(data)
	shaSum := sha256.Sum256(data)
	return hex.EncodeToString(md5Sum[:]), hex.EncodeToString(shaSum[:])
}

type tarEntry struct {
	Name     string
	Data     []byte
	Typeflag byte
}

func buildTar(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		typeflag := entry.Typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     entry.Name,
			Mode:     0o644,
			Typeflag: typeflag,
		}
		if typeflag == tar.TypeReg {
			hdr.Size = int64(len(entry.Data))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %s: %v", entry.Name, err)
		}
		if typeflag == tar.TypeReg {
			if _, err := tw.Write(entry.Data); err != nil {
				t.Fatalf("write tar body %s: %v", entry.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf.Bytes()
}

type mockStore struct {
	base      depotstore.Store
	open      func(name string) (fs.File, error)
	walkDir   func(root string, fn fs.WalkDirFunc) error
	readFile  func(name string) ([]byte, error)
	writeFile func(name string, data []byte) error
	stat      func(name string) (fs.FileInfo, error)
	mkdirAll  func(name string) error
	rename    func(oldName, newName string) error
	removeAll func(name string) error
}

var _ depotstore.Store = (*mockStore)(nil)

func newMockStore(t *testing.T) *mockStore {
	t.Helper()
	return &mockStore{base: depotstore.Dir(t.TempDir())}
}

func (m *mockStore) Open(name string) (fs.File, error) {
	if m.open != nil {
		return m.open(name)
	}
	if opener, ok := m.base.(interface{ Open(string) (fs.File, error) }); ok {
		return opener.Open(name)
	}
	return nil, fs.ErrInvalid
}

func (m *mockStore) WalkDir(root string, fn fs.WalkDirFunc) error {
	if m.walkDir != nil {
		return m.walkDir(root, fn)
	}
	return m.base.WalkDir(root, fn)
}

func (m *mockStore) ReadFile(name string) ([]byte, error) {
	if m.readFile != nil {
		return m.readFile(name)
	}
	return m.base.ReadFile(name)
}

func (m *mockStore) WriteFile(name string, data []byte) error {
	if m.writeFile != nil {
		return m.writeFile(name, data)
	}
	return m.base.WriteFile(name, data)
}

func (m *mockStore) Stat(name string) (fs.FileInfo, error) {
	if m.stat != nil {
		return m.stat(name)
	}
	return m.base.Stat(name)
}

func (m *mockStore) MkdirAll(name string) error {
	if m.mkdirAll != nil {
		return m.mkdirAll(name)
	}
	return m.base.MkdirAll(name)
}

func (m *mockStore) Rename(oldName, newName string) error {
	if m.rename != nil {
		return m.rename(oldName, newName)
	}
	return m.base.Rename(oldName, newName)
}

func (m *mockStore) RemoveAll(name string) error {
	if m.removeAll != nil {
		return m.removeAll(name)
	}
	return m.base.RemoveAll(name)
}
