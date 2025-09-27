package models

import (
	"context"
	"slices"
	"sort"
	"time"
)

type PlaybackStatus int

const (
	PlaybackStatusUnknown PlaybackStatus = iota
	PlaybackStatusPlaying
	PlaybackStatusPaused
	PlaybackStatusStopped
)

type LyricLine struct {
	Position int // milli
	Text     string
}

type Lyrics struct {
	Lines  []*LyricLine
	Source string
}

func (l *Lyrics) Len() int {
	return len(l.Lines)
}

func (l *Lyrics) IndexOf(position int, offset int) int {
	offPos := position - offset
	return sort.Search(len(l.Lines), func(i int) bool { return l.Lines[i].Position >= offPos }) - 1
}

func (l *Lyrics) Get(index int) string {
	if index < 0 || index >= len(l.Lines) {
		return ""
	}
	return l.Lines[index].Text
}

type MPRISMetadata struct {
	Title    string
	Artists  []string
	Text     string
	URL      string
	Duration time.Duration
	// Album    string
}

func (m *MPRISMetadata) Clone() MPRISMetadata {
	return MPRISMetadata{
		Title:    m.Title,
		Artists:  slices.Clone(m.Artists),
		Text:     m.Text,
		URL:      m.URL,
		Duration: m.Duration,
		// Album:    m.Album,
	}
}

type MPRISProperties struct {
	Metadata       MPRISMetadata
	Position       int
	PlaybackStatus PlaybackStatus
}

func (p *MPRISProperties) Clone() MPRISProperties {
	return MPRISProperties{
		Metadata:       p.Metadata.Clone(),
		Position:       p.Position,
		PlaybackStatus: p.PlaybackStatus,
	}
}

type Candidate struct {
	Titles   []string
	Artists  []string
	Duration time.Duration
	Lyrics   func(context.Context) (*Lyrics, error)
}
