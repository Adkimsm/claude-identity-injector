package main

import (
	_ "embed"
)

//go:embed webui/index.html
var statusPage []byte

func statusPageHTML() ([]byte, error) {
	copied := make([]byte, len(statusPage))
	copy(copied, statusPage)
	return copied, nil
}