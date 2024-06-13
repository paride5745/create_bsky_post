package post

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func UploadFile(ctx context.Context, pdsURL, accessToken, filename string, imgBytes []byte) (map[string]interface{}, error) {
	suffix := strings.ToLower(filename[strings.LastIndex(filename, ".")+1:])
	mimetype := "application/octet-stream"
	switch suffix {
	case "png":
		mimetype = "image/png"
	case "jpeg", "jpg":
		mimetype = "image/jpeg"
	case "webp":
		mimetype = "image/webp"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", pdsURL+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewBuffer(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", mimetype)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result["blob"].(map[string]interface{}), nil
}

func UploadImages(ctx context.Context, pdsURL, accessToken string, imagePaths []string, altText string) (map[string]interface{}, error) {
	var images []map[string]interface{}
	for _, ip := range imagePaths {
		imgBytes, err := os.ReadFile(ip)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", ip, err)
		}

		if len(imgBytes) > 1000000 {
			return nil, fmt.Errorf("image file size too large. 1000000 bytes maximum, got: %d", len(imgBytes))
		}

		blob, err := UploadFile(ctx, pdsURL, accessToken, ip, imgBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file %s: %w", ip, err)
		}

		images = append(images, map[string]interface{}{
			"image": blob,
			"alt":   altText,
		})
	}

	return map[string]interface{}{
		"$type":  "app.bsky.embed.images",
		"images": images,
	}, nil
}
