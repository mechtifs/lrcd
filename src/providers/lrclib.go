package providers

import (
	"context"
	"encoding/json/v2"
	"errors"
	"iter"
	"net/url"
	"time"

	"lrcd/models"
	"lrcd/utils"
)

type LRCLIBProvider struct{}

type LRCLIBResponse []*struct {
	TrackName    string        `json:"trackName"`
	ArtistName   string        `json:"artistName"`
	SyncedLyrics string        `json:"syncedLyrics"`
	Duration     time.Duration `json:"duration,format:sec"`
	// ID           int     `json:"id"`
	// AlbumName    string  `json:"albumName"`
	// Instrumental bool    `json:"instrumental"`
	// PlainLyrics  string  `json:"plainLyrics"`
}

func NewLRCLIBProvider() *LRCLIBProvider {
	return &LRCLIBProvider{}
}

func (*LRCLIBProvider) ID() string {
	return LRCLIBProviderID
}

func (p *LRCLIBProvider) IterAll(ctx context.Context, meta *models.MPRISMetadata) (iter.Seq[*models.Candidate], error) {
	return func(yield func(*models.Candidate) bool) {
		retFlag := false
		queryAll := func(title string) {
			for _, artist := range meta.Artists {
				resp, err := get(ctx, "https://lrclib.net/api/search?track_name="+url.QueryEscape(title)+"&artist_name="+url.QueryEscape(artist), "User-Agent", "lrcd")
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					continue
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					continue
				}
				body := LRCLIBResponse{}
				err = json.UnmarshalRead(resp.Body, &body)
				if err != nil {
					continue
				}
				for _, track := range body {
					candidate := &models.Candidate{
						Titles:   []string{track.TrackName},
						Artists:  []string{track.ArtistName},
						Duration: track.Duration,
						Lyrics: func(ctx context.Context) (*models.Lyrics, error) {
							if track.SyncedLyrics == "" {
								return nil, ErrNoLyrics
							}
							lines, err := utils.ParseLrc(track.SyncedLyrics)
							if err != nil {
								return nil, ErrParseFailure
							}
							return &models.Lyrics{
								Lines:  lines,
								Source: p.ID(),
							}, nil
						},
					}
					if !yield(candidate) {
						retFlag = true
						return
					}
				}
			}
		}
		queryAll(meta.Title)
		if retFlag {
			return
		}
		altTitle := utils.StripTitle(meta.Title)
		if altTitle != meta.Title {
			queryAll(altTitle)
		}
	}, nil
}
