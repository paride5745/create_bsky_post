package auth

import (
	"bytes"
	"context"
	"create_bsky_post/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func BskyLoginSession(ctx context.Context, pdsURL, handle, password string) (utils.Session, error) {
	var session utils.Session
	payload := map[string]string{"identifier": handle, "password": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return session, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", pdsURL+"/xrpc/com.atproto.server.createSession", bytes.NewBuffer(body))
	if err != nil {
		return session, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return session, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return session, errors.New(resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&session)
	if err != nil {
		return session, fmt.Errorf("failed to decode response: %w", err)
	}
	return session, nil
}
