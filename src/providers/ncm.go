package providers

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"iter"
	"strconv"
	"time"

	"lrcd/models"
	"lrcd/utils"
)

const NCMBaseURL = "https://music.163.com/api"

type NCMProvider struct{}

type NCMSong struct {
	ID         int           `json:"id"`
	Name       string        `json:"name"`
	Alias      []string      `json:"alias"`
	TransNames []string      `json:"transNames"`
	Duration   time.Duration `json:"duration,format:milli"`
	Artists    []*struct {
		Name  string   `json:"name"`
		Alias []string `json:"alias"`
		// ID    int      `json:"id"`
	} `json:"artists"`
	// Album struct {
	// 	ID   int    `json:"id"`
	// 	Name string `json:"name"`
	// }
}

type NCMSearchResponse struct {
	Result struct {
		Songs []*NCMSong `json:"songs"`
		// SongCount int       `json:"SongCount"`
	} `json:"result"`
}

type NCMGetResponse struct {
	Lrc struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
}

func NewNCMProvider() *NCMProvider {
	return &NCMProvider{}
}

func (*NCMProvider) ID() string {
	return NCMProviderID
}

func (p *NCMProvider) IterAll(ctx context.Context, meta *models.MPRISMetadata) (iter.Seq[*models.Candidate], error) {
	resp, err := get(ctx, NCMBaseURL+"/search/get/web?limit=30&type=1&s="+queryStr(meta))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := NCMSearchResponse{}
	// Sometimes ncm will return duplicated `alias` fields at /result/songs/<index>
	err = json.UnmarshalRead(resp.Body, &body, jsontext.AllowDuplicateNames(true))
	if err != nil {
		return nil, ErrParseFailure
	}
	return func(yield func(*models.Candidate) bool) {
		for _, track := range body.Result.Songs {
			titles := append(track.Alias, track.Name)
			artists := []string{}
			for _, a := range track.Artists {
				artists = append(artists, a.Name)
				artists = append(artists, a.Alias...)
			}
			candidate := &models.Candidate{
				Titles:   titles,
				Artists:  artists,
				Duration: track.Duration,
				Lyrics: func(ctx context.Context) (*models.Lyrics, error) {
					resp, err := get(ctx, NCMBaseURL+"/song/lyric?lv=1&id="+strconv.Itoa(track.ID))
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()
					body := NCMGetResponse{}
					err = json.UnmarshalRead(resp.Body, &body)
					if err != nil {
						return nil, ErrParseFailure
					}
					lines, err := utils.ParseLrc(body.Lrc.Lyric)
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
				return
			}
		}
	}, nil
}
