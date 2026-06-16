package main

import (
	"flag"
	"fmt"
	"os"

	"openwatcher/internal/releasehistory"
)

func main() {
	var (
		historyPath string
		outputPath  string
	)
	flag.StringVar(&historyPath, "history", "", "path to watch-app/RELEASE_BUILDS.md")
	flag.StringVar(&outputPath, "output", "", "path to output changelog json")
	flag.Parse()

	if historyPath == "" || outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/watch-release-changelog --history <path> --output <path>")
		os.Exit(2)
	}

	input, err := os.Open(historyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open history file: %v\n", err)
		os.Exit(1)
	}
	defer input.Close()

	entries, err := releasehistory.ParseMarkdown(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse history file: %v\n", err)
		os.Exit(1)
	}

	payload, err := releasehistory.EncodeUserChangelogJSON(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode changelog json: %v\n", err)
		os.Exit(1)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(outputPath, payload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write changelog json: %v\n", err)
		os.Exit(1)
	}
}
