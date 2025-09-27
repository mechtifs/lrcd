package providers

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"iter"
	"strconv"
	"time"

	"lrcd/models"
	"lrcd/utils"
)

const MXMBaseURL = "https://apic-desktop.musixmatch.com/ws/1.1"

type MXMProvider struct {
	token string
}

type MXMTrack struct {
	Track struct {
		ID           int           `json:"commontrack_id"`
		ArtistID     int           `json:"artist_id"`
		ArtistName   string        `json:"artist_name"`
		TrackName    string        `json:"track_name"`
		HasSubtitles int           `json:"has_subtitles"`
		TrackLength  time.Duration `json:"track_length,format:sec"`
		// AlbumName    string        `json:"album_name"`
	} `json:"track"`
}

type MXMResponseHeader struct {
	StatusCode int    `json:"status_code"`
	Hint       string `json:"hint"`
}

type MXMBaseResponse struct {
	Message struct {
		Header MXMResponseHeader `json:"header"`
		Body   jsontext.Value    `json:"body"`
	} `json:"message"`
}

type MXMTokenGetResponse struct {
	Message struct {
		Header MXMResponseHeader `json:"header"`
		Body   struct {
			UserToken string `json:"user_token"`
		} `json:"body"`
	} `json:"message"`
}

type MXMTrackSearchResponseBody struct {
	TrackList []*MXMTrack `json:"track_list"`
}

type MXMSubtitileGetResponseBody struct {
	Subtitle struct {
		SubtitleBody string `json:"subtitle_body"`
	} `json:"subtitle"`
}

type MXMArtistGetResponseBody struct {
	Artist struct {
		ArtistAliasList []*struct {
			ArtistAlias string `json:"artist_alias"`
		} `json:"artist_alias_list"`
	} `json:"artist"`
}

func NewMXMProvider() *MXMProvider {
	return &MXMProvider{}
}

func (*MXMProvider) ID() string {
	return MXMProviderID
}

func (p *MXMProvider) IterAll(ctx context.Context, meta *models.MPRISMetadata) (iter.Seq[*models.Candidate], error) {
	var err error
	if p.token == "" {
		err = p.updateToken(ctx)
		if err != nil {
			return nil, err
		}
	}
	b, err := p.getBody(ctx, "/track.search?page_size=10&page=1&q="+queryStr(meta))
	if err != nil {
		return nil, err
	}
	body := MXMTrackSearchResponseBody{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, ErrParseFailure
	}
	return func(yield func(*models.Candidate) bool) {
		artistsMap := map[int][]string{}
		for _, track := range body.TrackList {
			titles := []string{track.Track.TrackName}
			artists := []string{track.Track.ArtistName}
			if a, ok := artistsMap[track.Track.ArtistID]; ok {
				artists = a
			} else {
				b, err := p.getBody(ctx, "/artist.get?artist_id="+strconv.Itoa(track.Track.ArtistID))
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
				} else {
					body := MXMArtistGetResponseBody{}
					err = json.Unmarshal(b, &body)
					if err == nil {
						for _, a := range body.Artist.ArtistAliasList {
							artists = append(artists, a.ArtistAlias)
						}
					}
				}
				artistsMap[track.Track.ArtistID] = artists
			}
			candidate := &models.Candidate{
				Titles:   titles,
				Artists:  artists,
				Duration: track.Track.TrackLength,
				Lyrics: func(ctx context.Context) (*models.Lyrics, error) {
					b, err := p.getBody(ctx, "/track.subtitle.get?commontrack_id="+strconv.Itoa(track.Track.ID))
					if err != nil {
						return nil, err
					}
					body := MXMSubtitileGetResponseBody{}
					err = json.Unmarshal(b, &body)
					if err != nil {
						return nil, ErrParseFailure
					}
					lines, err := utils.ParseLrc(body.Subtitle.SubtitleBody)
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

func (p *MXMProvider) updateToken(ctx context.Context) error {
	resp, err := get(ctx, MXMBaseURL+"/token.get?app_id=web-desktop-app-v1.0", "Cookie", "AWSELB=unknown")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := MXMTokenGetResponse{}
	err = json.UnmarshalRead(resp.Body, &body)
	if err != nil {
		return ErrParseFailure
	}
	if body.Message.Header.StatusCode != 200 {
		return ErrRateLimit
	}
	p.token = body.Message.Body.UserToken
	return nil
}

func (p *MXMProvider) getBody(ctx context.Context, endpoint string) (jsontext.Value, error) {
	resp, err := get(ctx, MXMBaseURL+endpoint+"&app_id=web-desktop-app-v1.0&usertoken="+p.token, "Cookie", "AWSELB=unknown")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := MXMBaseResponse{}
	err = json.UnmarshalRead(resp.Body, &body)
	if err != nil {
		return nil, ErrParseFailure
	}
	if body.Message.Header.Hint == "captcha" || body.Message.Header.Hint == "renew" {
		err := p.updateToken(ctx)
		if err != nil {
			return nil, err
		}
		resp, err := get(ctx, endpoint+"&usertoken="+p.token,
			"Cookie", "AWSELB=unknown",
		)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		err = json.UnmarshalRead(resp.Body, &body)
		if err != nil {
			return nil, ErrParseFailure
		}
		return body.Message.Body, nil
	}
	return body.Message.Body, nil
}
