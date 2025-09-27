package providers

import (
	"context"
	"slices"
	"testing"
	"time"

	"lrcd/models"
	"lrcd/utils"
)

var meta = &models.MPRISMetadata{
	Title:    "春日影",
	Artists:  []string{"CRYCHIC"},
	Duration: time.Duration(4*time.Minute + 18*time.Second),
}

func checkCandidate(meta *models.MPRISMetadata, candidate *models.Candidate) bool {
	titleMatched := false
	for _, t := range candidate.Titles {
		if utils.StripTitle(t) == meta.Title {
			titleMatched = true
			break
		}
	}
	if !titleMatched {
		return false
	}
	if !slices.Contains(candidate.Artists, meta.Artists[0]) {
		return false
	}
	if (meta.Duration - candidate.Duration).Abs() > 2*time.Second {
		return false
	}
	return true
}

func testProvider(t *testing.T, prov Provider) {
	ctx := context.Background()
	iter, err := prov.IterAll(ctx, meta)
	if err != nil {
		t.Fatal(err)
	}
	for c := range iter {
		if !checkCandidate(meta, c) {
			continue
		}
		lyrics, err := c.Lyrics(ctx)
		if err != nil {
			t.Log(err)
			continue
		}
		t.Log(lyrics.Len())
		return
	}
	t.Fail()
}

func TestLRCLIB(t *testing.T) {
	testProvider(t, NewLRCLIBProvider())
}

func TestKugou(t *testing.T) {
	testProvider(t, NewKugouProvider())
}

func TestNCM(t *testing.T) {
	testProvider(t, NewNCMProvider())
}

func TestKuwo(t *testing.T) {
	testProvider(t, NewKuwoProvider())
}

func TestMXM(t *testing.T) {
	testProvider(t, NewMXMProvider())
}
