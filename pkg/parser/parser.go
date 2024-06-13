package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"create_bsky_post/pkg/utils"
)

func ParseMentions(text string) ([]utils.Mention, error) {
	mentionRegex := regexp.MustCompile(`\B@([\w.-]+)`)
	textBytes := []byte(text)
	var mentions []utils.Mention
	matches := mentionRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, m := range matches {
		mentions = append(mentions, utils.Mention{
			Start:  m[0],
			End:    m[1],
			Handle: string(textBytes[m[2]:m[3]]),
		})
	}
	return mentions, nil
}

func ParseURLs(text string) ([]utils.URLSpan, error) {
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	textBytes := []byte(text)
	var urls []utils.URLSpan
	matches := urlRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, m := range matches {
		urls = append(urls, utils.URLSpan{
			Start: m[0],
			End:   m[1],
			URL:   string(textBytes[m[0]:m[1]]),
		})
	}
	return urls, nil
}

func ParseFacets(ctx context.Context, pdsURL, text string) ([]utils.Facet, error) {
	var facets []utils.Facet
	mentions, err := ParseMentions(text)
	if err != nil {
		return facets, fmt.Errorf("failed to parse mentions: %w", err)
	}

	for _, m := range mentions {
		req, err := http.NewRequestWithContext(ctx, "GET", pdsURL+"/xrpc/com.atproto.identity.resolveHandle?handle="+m.Handle, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode == http.StatusBadRequest {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return facets, fmt.Errorf("failed to decode response: %w", err)
		}

		did, ok := result["did"].(string)
		if !ok {
			return facets, errors.New("did not found in response")
		}

		facets = append(facets, utils.Facet{
			Index: utils.Index{ByteStart: m.Start, ByteEnd: m.End},
			Features: []utils.Feature{
				{Type: "app.bsky.richtext.facet#mention", Did: did},
			},
		})
	}

	urls, err := ParseURLs(text)
	if err != nil {
		return facets, fmt.Errorf("failed to parse URLs: %w", err)
	}

	for _, u := range urls {
		facets = append(facets, utils.Facet{
			Index: utils.Index{ByteStart: u.Start, ByteEnd: u.End},
			Features: []utils.Feature{
				{Type: "app.bsky.richtext.facet#link", Uri: u.URL},
			},
		})
	}

	return facets, nil
}

func ParseURI(uri string) (map[string]string, error) {
	parts := strings.Split(uri, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("unhandled URI format: %s", uri)
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

func GetReplyRefs(ctx context.Context, pdsURL, parentURI string) (map[string]map[string]string, error) {
	uriParts, err := ParseURI(parentURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parent URI: %w", err)
	}

	resp, err := http.Get(pdsURL + "/xrpc/com.atproto.repo.getRecord?repo=" + uriParts["repo"] + "&collection=" + uriParts["collection"] + "&rkey=" + uriParts["rkey"])
	if err != nil {
		return nil, fmt.Errorf("failed to perform get record request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var parent map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&parent)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	root := parent
	if parentReply, ok := parent["value"].(map[string]interface{})["reply"].(map[string]interface{}); ok {
		rootURI := parentReply["root"].(map[string]interface{})["uri"].(string)
		rootParts := strings.Split(rootURI, "/")
		rootRepo, rootCollection, rootRkey := rootParts[2], rootParts[3], rootParts[4]

		resp, err = http.Get(pdsURL + "/xrpc/com.atproto.repo.getRecord?repo=" + rootRepo + "&collection=" + rootCollection + "&rkey=" + rootRkey)
		if err != nil {
			return nil, fmt.Errorf("failed to perform get root record request: %w", err)
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&root)
		if err != nil {
			return nil, fmt.Errorf("failed to decode root response: %w", err)
		}
	}

	return map[string]map[string]string{
		"root": {
			"uri": root["uri"].(string),
			"cid": root["cid"].(string),
		},
		"parent": {
			"uri": parent["uri"].(string),
			"cid": parent["cid"].(string),
		},
	}, nil
}
