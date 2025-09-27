package providers

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"iter"
	"time"

	"lrcd/models"
	"lrcd/utils"
)

const KugouBaseURL = "http://lyrics.kugou.com"

type KugouProvider struct{}

type KugouSinger struct {
	Name string `json:"name"`
	// ID   int    `json:"id"`
}

type KugouSong struct {
	Hash              string        `json:"hash"`
	SongName          string        `json:"songname"`
	OtherName         string        `json:"othername"`
	SongNameOriginal  string        `json:"songname_original"`
	OtherNameOriginal string        `json:"othername_original"`
	SingerName        string        `json:"singername"`
	Duration          time.Duration `json:"duration,format:sec"`
	// AlbumName         string `json:"album_name"`
}

type KugouSongSearchResponse struct {
	Data struct {
		Info []*KugouSong `json:"info"`
	} `json:"data"`
}

type KugouCandidate struct {
	ID        string `json:"id"`
	Accesskey string `json:"accesskey"`
}

type KugouLyricsSearchResponse struct {
	Candidates []*KugouCandidate `json:"candidates"`
}

type KugouLyricsDownloadResponse struct {
	Content string `json:"content"`
}

func NewKugouProvider() *KugouProvider {
	return &KugouProvider{}
}

func (*KugouProvider) ID() string {
	return KugouProviderID
}

func (p *KugouProvider) IterAll(ctx context.Context, meta *models.MPRISMetadata) (iter.Seq[*models.Candidate], error) {
	resp, err := get(ctx, "http://msearchcdn.kugou.com/api/v3/search/song?keyword="+queryStr(meta))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := KugouSongSearchResponse{}
	err = json.UnmarshalRead(resp.Body, &body)
	if err != nil {
		return nil, ErrParseFailure
	}
	return func(yield func(*models.Candidate) bool) {
		for _, track := range body.Data.Info {
			titles := []string{track.SongName, track.SongNameOriginal, track.OtherName, track.OtherNameOriginal}
			artists := []string{track.SingerName}
			candidate := &models.Candidate{
				Titles:   titles,
				Artists:  artists,
				Duration: track.Duration,
				Lyrics: func(ctx context.Context) (*models.Lyrics, error) {
					resp, err = get(ctx, KugouBaseURL+"/search?ver=1&man=yes&client=pc&hash="+track.Hash)
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()
					body := KugouLyricsSearchResponse{}
					err = json.UnmarshalRead(resp.Body, &body)
					if err != nil {
						return nil, ErrParseFailure
					}
					for _, candidate := range body.Candidates {
						resp, err = get(ctx, KugouBaseURL+"/download?ver=1&client=pc&id="+candidate.ID+"&accesskey="+candidate.Accesskey+"&fmt=lrc&charset=utf8")
						if err != nil {
							if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
								return nil, err
							}
							continue
						}
						defer resp.Body.Close()
						body := KugouLyricsDownloadResponse{}
						err = json.UnmarshalRead(resp.Body, &body)
						if err != nil {
							continue
						}
						bytes, err := base64.StdEncoding.DecodeString(body.Content)
						if err != nil {
							continue
						}
						lines, err := utils.ParseLrc(string(bytes))
						if err != nil {
							continue
						}
						if lines[0].Text == "纯音乐，请欣赏" {
							continue
						}
						return &models.Lyrics{
							Lines:  lines,
							Source: p.ID(),
						}, nil
					}
					return nil, ErrNoLyrics
				},
			}
			if !yield(candidate) {
				return
			}
		}
	}, nil
}
