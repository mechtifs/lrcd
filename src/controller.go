package main

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"lrcd/models"
	"lrcd/providers"
	"lrcd/publishers"
	"lrcd/utils"
)

const (
	ETX = "\x03"
	EOT = "\x04"
)

type PublisherEntry struct {
	publishers.Publisher
	ch        chan string
	Offset    int
	SentIndex int
}

func NewPublisherEntry(publisher publishers.Publisher, offset int) *PublisherEntry {
	p := &PublisherEntry{
		Publisher: publisher,
		ch:        make(chan string, 16),
		Offset:    offset,
		SentIndex: -1,
	}
	go func() {
		for txt := range p.ch {
			err := p.Publisher.Send(txt)
			if err != nil {
				slog.Error("failed to send", "error", err, "publisher", p.ID())
			}
		}
	}()
	return p
}

func (p *PublisherEntry) Send(txt string) {
	select {
	case p.ch <- txt:
	default:
	}
}

// We send pre-defined clear instruction to tell adapters that we're in inactive state
func (p *PublisherEntry) Clear() {
	p.ch <- ETX
}

func (p *PublisherEntry) Exit() {
	close(p.ch)
	p.Publisher.Send(EOT)
	p.Publisher.Exit()
}

type ProviderEntry struct {
	providers.Provider
}

func NewProviderEntry(provider providers.Provider) *ProviderEntry {
	return &ProviderEntry{
		Provider: provider,
	}
}

type Controller struct {
	propsCh       <-chan models.MPRISProperties
	providers     []*ProviderEntry
	fetchMode     FetchMode
	fetchTimeout  int
	publishers    []*PublisherEntry
	showTitle     bool
	filterMatcher *utils.Matcher
	urlMatcher    *utils.Matcher
	cache         *Cache
	lyrics        *models.Lyrics
	props         models.MPRISProperties
	position      int

	mu               sync.Mutex
	cancelTicking    context.CancelFunc
	cancelFetching   context.CancelFunc
	currentRequestID int
}

type ControllerOptions struct {
	propsCh      <-chan models.MPRISProperties
	providers    []*ProviderEntry
	publishers   []*PublisherEntry
	fetchMode    FetchMode
	fetchTimeout int
	filters      []string
	urlBlacklist []string
	showTitle    bool
	cacheDir     string
}

func NewController(opt *ControllerOptions) *Controller {
	var cache *Cache
	var filterMatcher *utils.Matcher
	var urlMatcher *utils.Matcher
	if opt.cacheDir != "" {
		cache = &Cache{path: opt.cacheDir}
	}
	if len(opt.filters) > 0 {
		filterMatcher = utils.NewStringMatcher(opt.filters)
	}
	if len(opt.urlBlacklist) > 0 {
		urlMatcher = utils.NewStringMatcher(opt.urlBlacklist)
	}
	return &Controller{
		propsCh:       opt.propsCh,
		providers:     opt.providers,
		publishers:    opt.publishers,
		fetchMode:     opt.fetchMode,
		fetchTimeout:  opt.fetchTimeout,
		showTitle:     opt.showTitle,
		filterMatcher: filterMatcher,
		urlMatcher:    urlMatcher,
		cache:         cache,
	}
}

func (c *Controller) checkCandidate(meta *models.MPRISMetadata, candidate *models.Candidate, altTitle string, artistSet map[string]struct{}) bool {
	titleMatched := false
	artistMatched := false
	for _, t := range candidate.Titles {
		if t == meta.Title || utils.StripTitle(t) == altTitle {
			titleMatched = true
			break
		}
	}
	if !titleMatched {
		return false
	}
	for _, a := range candidate.Artists {
		if _, ok := artistSet[a]; ok {
			artistMatched = true
			break
		}
	}
	if !artistMatched {
		return false
	}
	if (meta.Duration - candidate.Duration).Abs() > 2*time.Second {
		return false
	}
	return true
}

func (c *Controller) fetchFastest(ctx context.Context, meta *models.MPRISMetadata) *models.Lyrics {
	altTitle := utils.StripTitle(meta.Title)
	artistSet := map[string]struct{}{}
	for _, a := range meta.Artists {
		artistSet[a] = struct{}{}
	}
	trackname := utils.FormatTrack(meta)
	wg := sync.WaitGroup{}
	lyricsCh := make(chan *models.Lyrics, len(c.providers))
	for _, prov := range c.providers {
		slog.Info("fetching lyrics", "track", trackname, "source", prov.ID())
		wg.Go(func() {
			iter, err := prov.IterAll(ctx, meta)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.Warn("fetch canceled", "track", trackname)
				} else {
					slog.Warn(err.Error(), "track", trackname, "source", prov.ID())
				}
				return
			}
			for candidate := range iter {
				if !c.checkCandidate(meta, candidate, altTitle, artistSet) {
					continue
				}
				lyrics, err := candidate.Lyrics(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						slog.Warn("fetch canceled", "track", trackname)
						return
					}
					continue
				}
				lyricsCh <- lyrics
			}
		})
	}
	go func() {
		wg.Wait()
		close(lyricsCh)
	}()
	return <-lyricsCh
}

func (c *Controller) fetchFallback(ctx context.Context, meta *models.MPRISMetadata) *models.Lyrics {
	altTitle := utils.StripTitle(meta.Title)
	artistSet := map[string]struct{}{}
	for _, a := range meta.Artists {
		artistSet[a] = struct{}{}
	}
	trackname := utils.FormatTrack(meta)
	for _, prov := range c.providers {
		slog.Info("fetching lyrics", "track", trackname, "source", prov.ID())
		iter, err := prov.IterAll(ctx, meta)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Warn("fetch canceled", "track", trackname)
				return nil
			}
			slog.Warn(err.Error(), "track", trackname, "source", prov.ID())
			continue
		}
		for candidate := range iter {
			if !c.checkCandidate(meta, candidate, altTitle, artistSet) {
				continue
			}
			lyrics, err := candidate.Lyrics(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.Warn("fetch canceled", "track", trackname)
					return nil
				}
				continue
			}
			return lyrics
		}
	}
	return nil
}

func (c *Controller) fetchLyrics(meta *models.MPRISMetadata, reqID int) (*models.Lyrics, bool) {
	if c.cache != nil {
		lyrics, err := c.cache.Get(meta)
		if err == nil {
			slog.Info("got cache", "track", utils.FormatTrack(meta))
			return lyrics, false
		}
	}
	if meta.Text != "" {
		lines, err := utils.ParseLrc(meta.Text)
		if err == nil {
			return &models.Lyrics{
				Lines:  lines,
				Source: "mpris",
			}, false
		}
	}
	if len(c.providers) == 0 {
		return nil, false
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if c.fetchTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(c.fetchTimeout)*time.Millisecond)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	c.mu.Lock()
	if reqID != c.currentRequestID {
		cancel()
		c.mu.Unlock()
		slog.Info("request canceled", "track", utils.FormatTrack(meta))
		return nil, false
	}
	if c.cancelFetching != nil {
		c.cancelFetching()
	}
	c.cancelFetching = cancel
	c.mu.Unlock()

	switch c.fetchMode {
	case FetchModeFallback:
		return c.fetchFallback(ctx, meta), true
	case FetchModeFastest:
		return c.fetchFastest(ctx, meta), true
	}
	c.mu.Lock()
	if c.cancelFetching != nil {
		c.cancelFetching()
		c.cancelFetching = nil
	}
	c.mu.Unlock()
	return nil, false
}

func (c *Controller) timedSend() {
	c.mu.Lock()
	if c.cancelTicking != nil {
		c.cancelTicking()
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelTicking = cancel
	c.mu.Unlock()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer func() {
		if c.cancelTicking != nil {
			c.cancelTicking()
			c.cancelTicking = nil
		}
		ticker.Stop()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.lyrics == nil {
				c.mu.Unlock()
				return
			}
			c.position += 100
			allDone := true
			for _, p := range c.publishers {
				idx := c.lyrics.IndexOf(c.position, p.Offset)
				if idx < c.lyrics.Len()-1 {
					allDone = false
				}
				if idx == p.SentIndex {
					continue
				}
				p.SentIndex = idx
				p.Send(c.lyrics.Get(idx))
			}
			if allDone {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}
}

func (c *Controller) resetAll() {
	if c.cancelTicking != nil {
		c.cancelTicking()
		c.cancelTicking = nil
	}
	if c.cancelFetching != nil {
		c.cancelFetching()
		c.cancelFetching = nil
	}
	c.position = 0
	c.lyrics = nil
	for _, publisher := range c.publishers {
		publisher.SentIndex = -1
		publisher.Clear()
	}
}

func (c *Controller) setLyrics(lyrics *models.Lyrics) {
	if c.filterMatcher == nil {
		c.lyrics = lyrics
		return
	}
	filtered := slices.Clone(lyrics.Lines)
	n := 0
	for _, line := range filtered {
		if !c.filterMatcher.Contains([]byte(line.Text)) {
			filtered[n] = line
			n++
		}
	}
	c.lyrics = &models.Lyrics{
		Source: lyrics.Source,
		Lines:  filtered[:n],
	}
}

func (c *Controller) process(props models.MPRISProperties) {
	slog.Debug("process", "properties", props)
	c.mu.Lock()
	defer c.mu.Unlock()
	defer func() { c.props = props }()

	if props.PlaybackStatus == models.PlaybackStatusUnknown && props.Metadata.Title == "" {
		slog.Info("backend reset")
		c.resetAll()
	} else if !utils.Equal(&props.Metadata, &c.props.Metadata) {
		c.resetAll()
		if props.Metadata.Title == "" || len(props.Metadata.Artists) == 0 {
			slog.Info("invalid metadata")
			return
		}
		if c.urlMatcher != nil && c.urlMatcher.Contains([]byte(props.Metadata.URL)) {
			slog.Info("blacklisted", "url", props.Metadata.URL)
			return
		}
		trackStr := utils.FormatTrack(&props.Metadata)
		slog.Info("playback changed", "track", trackStr)
		if c.showTitle && props.Metadata.Title != "" {
			for _, p := range c.publishers {
				p.Send(trackStr)
			}
		}
		c.currentRequestID++
		currentReqID := c.currentRequestID
		go func() {
			lyrics, shouldCache := c.fetchLyrics(&props.Metadata, currentReqID)
			if c.cache != nil && lyrics != nil && shouldCache {
				slog.Info("set cache", "track", trackStr, "source", lyrics.Source)
				go c.cache.Set(&props.Metadata, lyrics)
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if currentReqID != c.currentRequestID {
				slog.Info("request discarded", "track", trackStr)
				return
			}
			if lyrics == nil {
				slog.Info("no lyrics available", "track", trackStr)
				return
			}
			slog.Info("got lyrics", "track", trackStr, "source", lyrics.Source)
			c.setLyrics(lyrics)
			if c.props.PlaybackStatus == models.PlaybackStatusPlaying {
				go c.timedSend()
			}
		}()
	} else if props.PlaybackStatus != c.props.PlaybackStatus {
		if props.PlaybackStatus == models.PlaybackStatusPlaying {
			slog.Info("playback started")
			for _, p := range c.publishers {
				if c.lyrics != nil && c.lyrics.IndexOf(c.position, p.Offset) != -1 {
					p.Send(c.lyrics.Get(p.SentIndex))
				} else if c.showTitle && props.Metadata.Title != "" {
					p.Send(utils.FormatTrack(&props.Metadata))
				}
			}
			go c.timedSend()
		} else {
			slog.Info("playback stopped")
			if c.cancelTicking != nil {
				c.cancelTicking()
				c.cancelTicking = nil
			}
			for _, p := range c.publishers {
				p.Clear()
			}
		}
	}
	if props.Position != c.props.Position {
		c.position = props.Position
	}
}

func (c *Controller) Serve() {
	for props := range c.propsCh {
		c.process(props)
	}
}

func (c *Controller) Exit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetAll()
	for _, p := range c.publishers {
		p.Exit()
	}
}
