package providers

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"lrcd/models"
)

// Provider ID should be within 6 bytes
const (
	LRCLIBProviderID = "lrclib"
	NCMProviderID    = "ncm"
	KugouProviderID  = "kugou"
	KuwoProviderID   = "kuwo"
	MXMProviderID    = "mxm"
)

var (
	ErrNetworkFailure = errors.New("network failure")
	ErrParseFailure   = errors.New("parse failure")
	ErrNoLyrics       = errors.New("no lyrics found")
	ErrRateLimit      = errors.New("rate limited")
)

type Provider interface {
	ID() string
	IterAll(context.Context, *models.MPRISMetadata) (iter.Seq[*models.Candidate], error)
}

func queryStr(meta *models.MPRISMetadata) string {
	strings.NewReplacer()
	builder := &strings.Builder{}
	builder.WriteString(meta.Title)
	for _, artist := range meta.Artists {
		builder.WriteByte(' ')
		builder.WriteString(artist)
	}
	return url.QueryEscape(builder.String())
}

func get(ctx context.Context, url string, headers ...string) (*http.Response, error) {
	var resp *http.Response
	var err error
	slog.Debug("http get", "url", url)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	for range 5 {
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode == 429 {
			return nil, ErrRateLimit
		}
		break
	}
	if err != nil {
		return nil, ErrNetworkFailure
	}
	return resp, nil
}
