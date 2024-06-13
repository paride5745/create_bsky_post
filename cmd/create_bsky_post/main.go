package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"create_bsky_post/pkg/auth"
	"create_bsky_post/pkg/parser"
	"create_bsky_post/pkg/post"
)

func main() {
	var handle, password, text, parentURI, imagePath, altText, customDateString string
	flag.StringVar(&handle, "handle", "", "BSky handle")
	flag.StringVar(&password, "password", "", "BSky password")
	flag.StringVar(&text, "text", "", "Text to post")
	flag.StringVar(&parentURI, "parentURI", "", "Parent URI for reply")
	flag.StringVar(&imagePath, "imagePath", "", "Path to image")
	flag.StringVar(&altText, "altText", "", "Alt text for image")
	flag.StringVar(&customDateString, "customDate", "", "Custom creation date (DD/MM/YYYY hh:mm)")

	flag.Parse()

	// If password is not provided via command line, retrieve from environment variable
	if password == "" {
		password = os.Getenv("BSKY_APP_PASSWORD")
	}

	if handle == "" || password == "" || text == "" {
		log.Fatalf("handle, password, and text are required")
	}

	pdsURL := "https://bsky.social"
	ctx := context.Background()

	session, err := auth.BskyLoginSession(ctx, pdsURL, handle, password)
	if err != nil {
		log.Fatalf("Failed to login: %v", err)
	}

	facets, err := parser.ParseFacets(ctx, pdsURL, text)
	if err != nil {
		log.Fatalf("Failed to parse facets: %v", err)
	}

	var parent map[string]map[string]string
	if parentURI != "" {
		parent, err = parser.GetReplyRefs(ctx, pdsURL, parentURI)
		if err != nil {
			log.Fatalf("Failed to get reply refs: %v", err)
		}
	}

	var embed map[string]interface{}
	if imagePath != "" {
		embed, err = post.UploadImages(ctx, pdsURL, session.AccessJwt, []string{imagePath}, altText)
		if err != nil {
			log.Fatalf("Failed to upload images: %v\n", err)
		}
	}

	var customDate time.Time
	if customDateString != "" {
		customDate, err = parseCustomDate(customDateString)
		if err != nil {
			log.Fatalf("Invalid custom date format: %v\n", err)
		}
	} else {
		customDate = time.Now() // Default to current time if custom date is not provided
	}

	result, err := post.PostToBsky(context.Background(), pdsURL, session.Did, session.AccessJwt, text, facets, parent, embed, customDate)
	if err != nil {
		log.Fatalf("Failed to post: %v", err)
	}

	fmt.Printf("Post successful: %v\n", result)
}

func parseCustomDate(dateStr string) (time.Time, error) {
	layout := "02/01/2006 15:04" // DD/MM/YYYY hh:mm format
	return time.Parse(layout, dateStr)
}
