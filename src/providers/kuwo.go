package providers

import (
	"context"
	"encoding/json/v2"
	"iter"
	"strings"
	"time"

	"lrcd/models"
)

const KuwoBaseURL = "https://kuwo.cn"

type KuwoProvider struct{}

type KuwoAbs struct {
	ID        string        `json:"DC_TARGETID"`
	Artist    string        `json:"ARTIST"`
	AArtist   string        `json:"AARTIST"`
	FArtist   string        `json:"FARTIST"`
	Name      string        `json:"NAME"`
	Alias     string        `json:"ALIAS"`
	SongName  string        `json:"SONGNAME"`
	FSongName string        `json:"FSONGNAME"`
	Duration  time.Duration `json:"DURATION,string,format:sec"`
	// Album     string `json:"ALBUM"`
}

type KuwoSearchResponse struct {
	AbsList []*KuwoAbs `json:"abslist"`
	// Total   int       `json:"TOTAL,string"`
}

type KuwoLrcLine struct {
	TimeSec   float64 `json:"time,string"`
	LineLyric string  `json:"lineLyric"`
}

type KuwoGetResponse struct {
	Data struct {
		LrcList []*KuwoLrcLine `json:"lrclist"`
	} `json:"data"`
}

func NewKuwoProvider() *KuwoProvider {
	return &KuwoProvider{}
}

func (*KuwoProvider) ID() string {
	return KuwoProviderID
}

func (p *KuwoProvider) parseLrcList(lrcList []*KuwoLrcLine) ([]*models.LyricLine, error) {
	lines := []*models.LyricLine{}
	repCnt := 0
	prevTime := -1.0
	for i, lrc := range lrcList {
		if i == 0 {
			continue
		}
		line := &models.LyricLine{
			Position: int(lrc.TimeSec * 1000),
			Text:     strings.TrimSpace(lrc.LineLyric),
		}
		if lrc.TimeSec == prevTime {
			lines[len(lines)-1] = line
			repCnt++
		} else {
			lines = append(lines, line)
		}
		prevTime = lrc.TimeSec
	}
	if repCnt > 1 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, ErrNoLyrics
	}
	return lines, nil
}

func (p *KuwoProvider) IterAll(ctx context.Context, meta *models.MPRISMetadata) (iter.Seq[*models.Candidate], error) {
	resp, err := get(ctx, KuwoBaseURL+"/search/searchMusicBykeyWord?vipver=1&client=kt&ft=music&cluster=0&strategy=2012&encoding=utf8&rformat=json&mobi=1&issubtitle=1&pn=0&rn=20&all="+queryStr(meta))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := KuwoSearchResponse{}
	err = json.UnmarshalRead(resp.Body, &body)
	if err != nil {
		return nil, ErrParseFailure
	}
	return func(yield func(*models.Candidate) bool) {
		for _, track := range body.AbsList {
			titles := []string{track.Name, track.Alias, track.SongName, track.FSongName}
			artists := []string{}
			artists = append(artists, strings.Split(track.Artist, "&")...)
			artists = append(artists, strings.Split(track.AArtist, "&")...)
			artists = append(artists, strings.Split(track.FArtist, "&")...)
			candidate := &models.Candidate{
				Titles:   titles,
				Artists:  artists,
				Duration: track.Duration,
				Lyrics: func(ctx context.Context) (*models.Lyrics, error) {
					resp, err := get(ctx, KuwoBaseURL+"/openapi/v1/www/lyric/getlyric?musicId="+track.ID)
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()
					body := KuwoGetResponse{}
					err = json.UnmarshalRead(resp.Body, &body)
					if err != nil {
						return nil, ErrParseFailure
					}
					if len(body.Data.LrcList) == 0 {
						return nil, ErrNoLyrics
					}
					lines, err := p.parseLrcList(body.Data.LrcList)
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
