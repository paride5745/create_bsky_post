package post

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"create_bsky_post/pkg/utils"
)

func PostToBsky(ctx context.Context, pdsURL, repo, accessToken, text string, facets []utils.Facet, parent map[string]map[string]string, embed map[string]interface{}) (map[string]interface{}, error) {
	now := time.Now().Format(time.RFC3339)
	post := map[string]interface{}{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": now,
	}

	if len(facets) > 0 {
		post["facets"] = facets
	}

	if len(parent) > 0 {
		post["reply"] = map[string]map[string]string{
			"root":   parent["root"],
			"parent": parent["parent"],
		}
	}

	if embed != nil {
		post["embed"] = embed
	}

	body, err := json.Marshal(map[string]interface{}{
		"repo":       repo,
		"collection": "app.bsky.feed.post",
		"record":     post,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal post body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", pdsURL+"/xrpc/com.atproto.repo.createRecord", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response status: %s", resp.Status)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}
