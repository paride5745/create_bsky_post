package utils

import "net/http"

var HttpClient = &http.Client{}

type Index struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type Feature struct {
	Type string `json:"$type"`
	Did  string `json:"did,omitempty"`
	Uri  string `json:"uri,omitempty"`
}

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
