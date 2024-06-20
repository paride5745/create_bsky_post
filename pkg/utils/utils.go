package utils

import (
	"errors"
	"io"
	"strings"
)

func ParseURI(uri string) (map[string]string, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 5 {
		return nil, errors.New("unhandled URI format")
	}

	result := map[string]string{
		"repo":       parts[2],
		"collection": parts[3],
		"rkey":       parts[4],
	}

	if strings.HasPrefix(uri, "https://bsky.app/") {
		switch parts[3] {
		case "post":
			result["collection"] = "app.bsky.feed.post"
		case "lists":
			result["collection"] = "app.bsky.graph.list"
		case "feed":
			result["collection"] = "app.bsky.feed.generator"
		}
	}

	return result, nil
}

// NopCloser is used to convert a string reader to an io.ReadCloser
func NopCloser(r io.Reader) io.ReadCloser {
	return io.NopCloser(r)
}
