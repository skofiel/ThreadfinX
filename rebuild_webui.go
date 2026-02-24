//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	htmlDir := "html"
	outFile := filepath.Join("src", "webUI.go")
	mapName := "webUI"

	var entries []string

	err := filepath.Walk(htmlDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		// Use forward slashes and prepend extra component to match original key format
		// The original code does: filepath.Walk("html/") then uses getLocalPath which
		// strips the parent dir. Keys end up like "html/css/screen.css" -> "css/screen.css"
		// But let's check the existing webUI.go to see the actual key format
		key := strings.ReplaceAll(path, string(os.PathSeparator), "/")
		entries = append(entries, fmt.Sprintf("  %s[\"%s\"] = \"%s\"", mapName, key, b64))
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking html dir: %v\n", err)
		os.Exit(1)
	}

	// Check existing webUI.go to determine key format
	// Read the first few keys to see the pattern
	existingData, _ := os.ReadFile(outFile)
	existingStr := string(existingData)
	// Check if keys start with "html/" or not
	usesHTMLPrefix := strings.Contains(existingStr, `webUI["html/`)

	if !usesHTMLPrefix {
		// Strip "html/" prefix from keys
		for i, e := range entries {
			entries[i] = strings.Replace(e, `["`+htmlDir+`/`, `["`, 1)
		}
	}

	var content strings.Builder
	content.WriteString("package src\n\n")
	content.WriteString("var " + mapName + " = make(map[string]interface{})\n\n")
	content.WriteString("func loadHTMLMap() {\n\n")
	for _, entry := range entries {
		content.WriteString(entry + "\n")
	}
	content.WriteString("\n}\n\n")

	f, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	w.WriteString(content.String())
	w.Flush()

	fmt.Printf("Generated %s with %d entries\n", outFile, len(entries))
}
