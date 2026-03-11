package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func escapedSubpath(r *http.Request, prefix string) string {
	return strings.TrimPrefix(r.URL.EscapedPath(), prefix)
}

func decodeEscapedPath(path string) (string, error) {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("invalid escaped path: %w", err)
	}
	return decoded, nil
}

func decodeEscapedSegments(path string, want int) ([]string, error) {
	parts := strings.Split(path, "/")
	if len(parts) != want {
		return nil, fmt.Errorf("unexpected path segment count")
	}
	for i, part := range parts {
		decoded, err := decodeEscapedPath(part)
		if err != nil {
			return nil, err
		}
		parts[i] = decoded
	}
	return parts, nil
}
