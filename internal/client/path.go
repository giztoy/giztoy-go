package client

import (
	"net/url"
	"strings"
)

func escapePathSegment(value string) string {
	return url.PathEscape(value)
}

func joinEscapedPath(segments ...string) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, escapePathSegment(segment))
	}
	return strings.Join(escaped, "/")
}
