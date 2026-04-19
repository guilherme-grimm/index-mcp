// Package pathutil centralizes path resolution for MCP tool arguments.
package pathutil

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Resolve normalizes input (absolute or root-relative) against root and
// guarantees the result stays under root. root must already be absolute and
// cleaned (caller's responsibility).
func Resolve(root, input string) (string, error) {
	if input == "" {
		return "", errors.New("path is empty")
	}
	var clean string
	if filepath.IsAbs(input) {
		clean = filepath.Clean(input)
	} else {
		clean = filepath.Clean(filepath.Join(root, input))
	}
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return "", fmt.Errorf("path %q escapes root %q: %w", input, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", input, root)
	}
	return clean, nil
}
