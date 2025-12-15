package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndBuild(t *testing.T) {
	// Setup temporary output directory
	tempDir, err := os.MkdirTemp("", "ppt_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize PPTSubagent with the temp directory
	agent := NewPPTSubagent(nil, "gpt-4o", true, nil, tempDir)

	// Create sample slides
	slides := []Slide{
		{
			Title:   "Test Presentation",
			Content: []string{"Welcome to the test", "This is a bullet point"},
			Layout:  "title-center",
		},
		{
			Title:   "Slide 2",
			Content: []string{"Content for slide 2"},
			Layout:  "default",
		},
		{
			Title:   "Image Slide",
			Content: []string{"This slide has an image"},
			Image:   "https://picsum.photos/800/600",
			Layout:  "split-image-right",
		},
	}

	// Run GenerateAndBuild
	fmt.Println("Starting GenerateAndBuild test...")
	url, err := agent.GenerateAndBuild(context.Background(), slides)
	if err != nil {
		t.Fatalf("GenerateAndBuild failed: %v", err)
	}

	fmt.Printf("Successfully generated PPT at URL: %s\n", url)

	// Verify that the output directory contains the built files
	// The URL is like /generated/ppt_<timestamp>/dist/index.html
	// We need to find the actual directory in tempDir

	// List files in tempDir to find the created ppt directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	var pptDir string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[:4] == "ppt_" {
			pptDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}

	if pptDir == "" {
		t.Fatalf("Could not find generated ppt directory in %s", tempDir)
	}

	// Check for dist/index.html
	indexPath := filepath.Join(pptDir, "dist", "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("Expected index.html to exist at %s, but it does not", indexPath)
	} else {
		fmt.Printf("Found index.html at %s\n", indexPath)
	}
}
