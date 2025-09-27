package main

import (
	"context"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"lrcd/models"

	"github.com/godbus/dbus/v5"
)

type MPRIS struct {
	propsCh       chan<- models.MPRISProperties
	props         models.MPRISProperties
	mu            sync.Mutex
	cancelChecker context.CancelFunc
	conn          *dbus.Conn
	debouncer     *time.Timer
}

func NewMPRIS(propsCh chan<- models.MPRISProperties, conn *dbus.Conn) *MPRIS {
	return &MPRIS{
		propsCh: propsCh,
		conn:    conn,
	}
}

func (m *MPRIS) Serve() {
	go m.startChecker()
	m.listenSignals()
}

func (m *MPRIS) listenSignals() {
	err := m.conn.AddMatchSignal(
		dbus.WithMatchPathNamespace("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	if err != nil {
		log.Fatal("Failed to add match signal:", err)
	}
	err = m.conn.AddMatchSignal(
		dbus.WithMatchPathNamespace("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.mpris.MediaPlayer2.Player"),
		dbus.WithMatchMember("Seeked"),
	)
	if err != nil {
		log.Fatal("Failed to add match signal:", err)
	}

	c := make(chan *dbus.Signal, 8)
	m.conn.Signal(c)
	for signal := range c {
		m.mu.Lock()
		switch signal.Name {
		case "org.freedesktop.DBus.Properties.PropertiesChanged":
			m.onPropertiesChanged(signal)
		case "org.mpris.MediaPlayer2.Player.Seeked":
			m.onSeeked(signal)
		}
		props := m.props.Clone()
		m.mu.Unlock()
		// Sometimes more than one signal are emitted to fully update metadata (eg. kdeconnect)
		// So we add a small delay before sending the props we maintain
		if m.debouncer != nil {
			m.debouncer.Stop()
		}
		m.debouncer = time.AfterFunc(20*time.Millisecond, func() {
			m.debouncer = nil
			m.propsCh <- props
		})
	}
}

func (m *MPRIS) getBackends() []dbus.BusObject {
	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	var names []string
	var backends []dbus.BusObject
	call := obj.Call("org.freedesktop.DBus.ListNames", 0)
	if call.Err != nil {
		return []dbus.BusObject{}
	}
	call.Store(&names)
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			backends = append(backends, m.conn.Object(name, "/org/mpris/MediaPlayer2"))
		}
	}
	return backends
}

func (m *MPRIS) updatePosition(obj dbus.BusObject) {
	var position int64
	call := obj.Call(
		"org.freedesktop.DBus.Properties.Get", 0,
		"org.mpris.MediaPlayer2.Player", "Position",
	)
	if call.Err != nil {
		slog.Warn(call.Err.Error())
		m.mu.Lock()
		if m.cancelChecker != nil {
			m.cancelChecker()
			m.cancelChecker = nil
		}
		m.mu.Unlock()
		m.propsCh <- models.MPRISProperties{}
		return
	}
	call.Store(&position)
	m.mu.Lock()
	m.props.Position = int(position / 1000)
	props := m.props.Clone()
	m.mu.Unlock()
	m.propsCh <- props
}

func (m *MPRIS) startChecker() {
	var ctx context.Context
	var cancel context.CancelFunc
	var focus dbus.BusObject
	for _, backend := range m.getBackends() {
		call := backend.Call(
			"org.freedesktop.DBus.Properties.GetAll", 0,
			"org.mpris.MediaPlayer2.Player",
		)
		if call.Err != nil || len(call.Body) == 0 {
			continue
		}
		properties := parseProperties(call.Body[0].(map[string]dbus.Variant))

		if properties.PlaybackStatus != models.PlaybackStatusPlaying {
			continue
		}
		ctx, cancel = context.WithCancel(context.Background())
		m.propsCh <- properties.Clone()
		m.props = properties
		focus = backend
		break
	}
	if cancel == nil {
		return
	}
	m.cancelChecker = cancel
	m.updatePosition(focus)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updatePosition(focus)
		}
	}
}

func (m *MPRIS) onPropertiesChanged(signal *dbus.Signal) {
	if len(signal.Body) < 2 {
		return
	}
	p := signal.Body[1].(map[string]dbus.Variant)

	if md, ok := p["Metadata"]; ok {
		metadata := parseMetadata(md.Value().(map[string]dbus.Variant))
		m.props.Metadata = metadata
	}
	if ps, ok := p["PlaybackStatus"]; ok {
		var playbackStatus string
		ps.Store(&playbackStatus)
		m.props.PlaybackStatus = parsePlaybackStatus(playbackStatus)
		if m.cancelChecker != nil {
			m.cancelChecker()
			m.cancelChecker = nil
		}
		if m.props.PlaybackStatus == models.PlaybackStatusPlaying {
			go m.startChecker()
		}
	}
}

func (m *MPRIS) onSeeked(signal *dbus.Signal) {
	if len(signal.Body) == 0 {
		return
	}
	m.props.Position = int(signal.Body[0].(int64) / 1000)
}

func (m *MPRIS) Exit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelChecker != nil {
		m.cancelChecker()
		m.cancelChecker = nil
	}
}

func parsePlaybackStatus(ps string) models.PlaybackStatus {
	switch ps {
	case "Playing":
		return models.PlaybackStatusPlaying
	case "Paused":
		return models.PlaybackStatusPaused
	case "Stopped":
		return models.PlaybackStatusStopped
	}
	return models.PlaybackStatusUnknown
}

func parseMetadata(m map[string]dbus.Variant) models.MPRISMetadata {
	meta := models.MPRISMetadata{}
	if title, ok := m["xesam:title"]; ok {
		meta.Title = strings.TrimSpace(title.Value().(string))
	}
	if artists, ok := m["xesam:artist"]; ok {
		meta.Artists = artists.Value().([]string)
	}
	if text, ok := m["xesam:asText"]; ok {
		meta.Text = text.Value().(string)
	}
	if url, ok := m["xesam:url"]; ok {
		meta.URL = url.Value().(string)
	}
	if duration, ok := m["mpris:length"]; ok {
		meta.Duration = time.Duration(duration.Value().(int64)) * time.Microsecond
	}
	// if album, ok := m["xesam:album"]; ok {
	// 	meta.Album = strings.TrimSpace(album.Value().(string))
	// }
	return meta
}

func parseProperties(p map[string]dbus.Variant) models.MPRISProperties {
	props := models.MPRISProperties{}
	if metadata, ok := p["Metadata"]; ok {
		props.Metadata = parseMetadata(metadata.Value().(map[string]dbus.Variant))
	}
	if position, ok := p["Position"]; ok {
		props.Position = int(position.Value().(int64) / 1000)
	}
	if playbackStatus, ok := p["PlaybackStatus"]; ok {
		props.PlaybackStatus = parsePlaybackStatus(playbackStatus.Value().(string))
	}
	return props
}
