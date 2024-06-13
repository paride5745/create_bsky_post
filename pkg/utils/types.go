package utils

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
