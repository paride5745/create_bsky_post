package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Session struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
}

type Mention struct {
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Handle string `json:"handle"`
}

type URLSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	URL   string `json:"url"`
}

type Facet struct {
	Index    Index     `json:"index"`
	Features []Feature `json:"features"`
}

type Index struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type Feature struct {
	Type string `json:"$type"`
	Did  string `json:"did,omitempty"`
	Uri  string `json:"uri,omitempty"`
}

var httpClient = &http.Client{}

func bskyLoginSession(pdsURL, handle, password string) (Session, error) {
	var session Session
	payload := map[string]string{"identifier": handle, "password": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return session, err
	}

	resp, err := http.Post(pdsURL+"/xrpc/com.atproto.server.createSession", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return session, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return session, errors.New(resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&session)
	return session, err
}

func parseMentions(text string) ([]Mention, error) {
	mentionRegex := regexp.MustCompile(`\B@([\w.-]+)`)
	textBytes := []byte(text)
	var mentions []Mention
	matches := mentionRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, m := range matches {
		mentions = append(mentions, Mention{
			Start:  m[0],
			End:    m[1],
			Handle: string(textBytes[m[2]:m[3]]),
		})
	}
	return mentions, nil
}

func parseURLs(text string) ([]URLSpan, error) {
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	textBytes := []byte(text)
	var urls []URLSpan
	matches := urlRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, m := range matches {
		urls = append(urls, URLSpan{
			Start: m[0],
			End:   m[1],
			URL:   string(textBytes[m[0]:m[1]]),
		})
	}
	return urls, nil
}

func parseFacets(pdsURL, text string) ([]Facet, error) {
	var facets []Facet
	mentions, err := parseMentions(text)
	if err != nil {
		return facets, err
	}

	for _, m := range mentions {
		resp, err := httpClient.Get(pdsURL + "/xrpc/com.atproto.identity.resolveHandle?handle=" + m.Handle)
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
			return facets, err
		}

		did, ok := result["did"].(string)
		if !ok {
			return facets, errors.New("did not found in response")
		}

		facets = append(facets, Facet{
			Index: Index{ByteStart: m.Start, ByteEnd: m.End},
			Features: []Feature{
				{Type: "app.bsky.richtext.facet#mention", Did: did},
			},
		})
	}

	urls, err := parseURLs(text)
	if err != nil {
		return facets, err
	}

	for _, u := range urls {
		facets = append(facets, Facet{
			Index: Index{ByteStart: u.Start, ByteEnd: u.End},
			Features: []Feature{
				{Type: "app.bsky.richtext.facet#link", Uri: u.URL},
			},
		})
	}

	return facets, nil
}

func parseURI(uri string) (map[string]string, error) {
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

func getReplyRefs(pdsURL, parentURI string) (map[string]map[string]string, error) {
	uriParts, err := parseURI(parentURI)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(pdsURL + "/xrpc/com.atproto.repo.getRecord?repo=" + uriParts["repo"] + "&collection=" + uriParts["collection"] + "&rkey=" + uriParts["rkey"])
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parent map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&parent)
	if err != nil {
		return nil, err
	}

	root := parent
	if parentReply, ok := parent["value"].(map[string]interface{})["reply"].(map[string]interface{}); ok {
		rootURI := parentReply["root"].(map[string]interface{})["uri"].(string)
		rootParts := strings.Split(rootURI, "/")
		rootRepo, rootCollection, rootRkey := rootParts[2], rootParts[3], rootParts[4]

		resp, err = httpClient.Get(pdsURL + "/xrpc/com.atproto.repo.getRecord?repo=" + rootRepo + "&collection=" + rootCollection + "&rkey=" + rootRkey)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&root)
		if err != nil {
			return nil, err
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

func uploadFile(pdsURL, accessToken, filename string, imgBytes []byte) (map[string]interface{}, error) {
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

	req, err := http.NewRequest("POST", pdsURL+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewBuffer(imgBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimetype)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result["blob"].(map[string]interface{}), err
}

func uploadImages(pdsURL, accessToken string, imagePaths []string, altText string) (map[string]interface{}, error) {
	var images []map[string]interface{}
	for _, ip := range imagePaths {
		imgBytes, err := os.ReadFile(ip)
		if err != nil {
			return nil, err
		}

		if len(imgBytes) > 1000000 {
			return nil, fmt.Errorf("image file size too large. 1000000 bytes maximum, got: %d", len(imgBytes))
		}

		blob, err := uploadFile(pdsURL, accessToken, ip, imgBytes)
		if err != nil {
			return nil, err
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

func postToBsky(pdsURL, repo, accessToken, text string, facets []Facet, parent map[string]map[string]string, embed map[string]interface{}) (map[string]interface{}, error) {
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
		return nil, err
	}

	req, err := http.NewRequest("POST", pdsURL+"/xrpc/com.atproto.repo.createRecord", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func main() {
	var handle, password, text, parentURI, imagePath, altText string
	flag.StringVar(&handle, "handle", "", "BSky handle")
	flag.StringVar(&password, "password", "", "BSky password")
	flag.StringVar(&text, "text", "", "Text to post")
	flag.StringVar(&parentURI, "parentURI", "", "Parent URI for reply")
	flag.StringVar(&imagePath, "imagePath", "", "Path to image")
	flag.StringVar(&altText, "altText", "", "Alt text for image")
	flag.Parse()

	if handle == "" || password == "" || text == "" {
		fmt.Println("handle, password, and text are required")
		os.Exit(1)
	}

	pdsURL := "https://bsky.social"
	session, err := bskyLoginSession(pdsURL, handle, password)
	if err != nil {
		fmt.Printf("Failed to login: %v\n", err)
		os.Exit(1)
	}

	facets, err := parseFacets(pdsURL, text)
	if err != nil {
		fmt.Printf("Failed to parse facets: %v\n", err)
		os.Exit(1)
	}

	var parent map[string]map[string]string
	if parentURI != "" {
		parent, err = getReplyRefs(pdsURL, parentURI)
		if err != nil {
			fmt.Printf("Failed to get reply refs: %v\n", err)
			os.Exit(1)
		}
	}

	var embed map[string]interface{}
	if imagePath != "" {
		embed, err = uploadImages(pdsURL, session.AccessJwt, []string{imagePath}, altText)
		if err != nil {
			fmt.Printf("Failed to upload images: %v\n", err)
			os.Exit(1)
		}
	}

	result, err := postToBsky(pdsURL, session.Did, session.AccessJwt, text, facets, parent, embed)
	if err != nil {
		fmt.Printf("Failed to post: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Post successful: %v\n", result)
}
