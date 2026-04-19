package pathutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	root := filepath.Clean("/tmp/proj")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"absolute under root", "/tmp/proj/app/main.go", "/tmp/proj/app/main.go", false},
		{"relative under root", "app/main.go", "/tmp/proj/app/main.go", false},
		{"absolute outside root", "/etc/passwd", "", true},
		{"dot-dot escape", "../other/file.go", "", true},
		{"empty input", "", "", true},
		{"root itself", ".", root, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(root, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
			if !strings.HasPrefix(got, root) {
				t.Fatalf("resolved path %q not under root %q", got, root)
			}
		})
	}
}
