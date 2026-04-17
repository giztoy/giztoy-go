package adminui

import (
	"io/fs"
	"testing"
)

func TestFS(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "app.css"} {
		data, err := fs.ReadFile(FS(), name)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}
