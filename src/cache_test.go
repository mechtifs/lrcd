package main

import (
	"os"
	"path/filepath"
	"testing"

	"lrcd/models"
)

func TestCacheGet(t *testing.T) {
	baseDir, _ := os.UserCacheDir()
	cacheDir := filepath.Join(baseDir, "lrcd")
	cache := &Cache{path: cacheDir}
	meta := &models.MPRISMetadata{
		Title:   "春日影",
		Artists: []string{"CRYCHIC"},
	}
	lyrics, err := cache.Get(meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lyrics.Lines {
		t.Logf("[%02d:%02d.%03d] %s\n", line.Position/60_000, line.Position/1000%60, line.Position%1000, line.Text)
	}
}
