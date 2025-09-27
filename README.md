# lrcd

A modern, high-performance lyrics daemon that automatically fetches and displays synchronized lyrics for your currently playing music across different desktop environments.

## Features

- **Multi-Provider Support**: Fetches lyrics from multiple sources
  - [Musixmatch](https://www.musixmatch.com/)
  - [LRCLIB](https://liblrc.net/)
  - [NetEase Cloud Music](https://music.163.com/)
  - [Kugou Music](https://www.kugou.com/)
  - [Kuwo Music](https://kuwo.cn/)

- **Multi-Platform Publishers**: Output lyrics to various targets
  - File output
  - D-Bus messages
  - WebSocket
  - HTTP requests

- **And…**
  - Easy integration for desktop environments
  - Automatic lyrics caching with LZ4 compression
  - Real-time synchronized display
  - Multiple fetch strategies
  - Content filtering support
  - URL blacklist support
  - Configurable time offsets per publisher
  - …

## Installation

### Prerequisites

- Go 1.25+ with `GOEXPERIMENT=jsonv2`
- D-Bus (for MPRIS integration)

### Build from Source

```bash
git clone https://github.com/mechtifs/lrcd.git
cd lrcd/src
GOEXPERIMENT=jsonv2 go build -ldflags="-s -w" -o lrcd .
```

### Install Binary

```bash
# Install locally (preferred)
mkdir -p ~/.local/bin  # make sure `~/.local/bin` is in your $PATH
cp lrcd ~/.local/bin/

# Install globally
sudo cp lrcd /usr/local/bin/
```

## Configuration

lrcd uses a YAML configuration file located at `~/.config/lrcd/config.yaml`.

### Basic Configuration

```yaml
# Fetch strategy: "fallback" or "fastest"
fetch_mode: "fallback"

# Timeout for lyrics fetching (milliseconds)
fetch_timeout: 10000

# Enable lyrics caching
use_cache: true

# Show track title when no lyrics available
show_title: true

# Log level: "debug", "info", "warn", "error"
log_level: "info"

# Providers (in priority order for fallback mode)
providers:
  - id: mxm
  - id: lrclib
  - id: ncm
  - id: kugou
  - id: kuwo

# Publishers (output targets)
publishers:
  # File Publisher
  - id: file
    offset: 0
    options:
      path: "/dev/stdout"  # If ends with ".pipe", lrcd will try to create a pipe if not exists
      format: "\x1b[32m[+] %s\x1b[0m\n"

  # D-Bus Publisher
  - id: dbus
    offset: -100
    options:
      path: /com/github/mechtifs/lrcd
      name: com.github.mechtifs.lrcd.Updated

  # WebSocket Publisher
  - id: websocket
    offset: -250
    options:
      address: 127.0.0.1:5723

  # HTTP Publisher
  - id: http
    offset: -250
    options:
      method: PUT
      url: http://127.0.0.1:9999

# URL blacklist (skip lyrics for these URLs, mainly designed to skip videos since there's no way to tell the media type)
url_blacklist:
  - youtube.com/watch
  - bilibili.com

# Content filters (ignore lines containing these patterns)
filters:
  - "作词"
  - "作曲"
  - "编曲"
  - "词："
  - "曲："
```

## Usage

### Start the Daemon

```bash
# Run in foreground
lrcd

# Run in background
lrcd &
```

### Systemd Service

Create `~/.config/systemd/user/lrcd.service`:

```ini
[Unit]
Description=lrcd - Lyrics Daemon
After=graphical-session.target

[Service]
Type=simple
ExecStart=%h/.local/bin/lrcd
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

Enable and start:

```bash
systemctl --user daemon-reload
systemctl --user enable --now lrcd
```

## Desktop Integration

### GNOME Extension

1. Copy the extension to GNOME extensions directory:
```bash
cp -r adapters/gnome/* ~/.local/share/gnome-shell/extensions
```

2. Enable the extension via GNOME Extensions

> If you are using [AGS](https://aylur.github.io/ags/), the code should be roughly the same.

### KDE Plasmoid

1. Install the plasmoid:
```bash
cp -r adapters/kde/* ~/.local/share/plasma/plasmoids
```

2. Add the widget to panel or desktop in edit mode

> If you are using [QuickShell](https://quickshell.org/), the code should be roughly the same.

## Development

### Project Structure

```
lrcd/
├── src/
│   ├── main.go          # Entry point
│   ├── config.go        # Configuration parsing
│   ├── controller.go    # Main controller logic
│   ├── cache.go         # Lyrics caching
│   ├── mpris.go         # MPRIS integration
│   ├── models/          # Data models
│   ├── providers/       # Lyrics providers
│   ├── publishers/      # Output publishers
│   └── utils/           # Utility functions
└── adapters/            # Desktop environment adapters
    ├── gnome/           # GNOME Shell extension
    └── kde/             # KDE plasmoid
```

### Cache Format

lrcd uses a custom binary format with LZ4 compression for efficient lyrics storage.

```
Cache File (.cache)
├── Header (16 bytes)
│   ├── Signature: [4]byte "lrcd"
│   ├── Body Size: uint32 (uncompressed size)
│   ├── Line Count: uint16
│   └── Source: [6]byte (provider ID)
└── Body (LZ4 compressed)
    └── Lyrics Lines (repeated)
        ├── Position: uint32 (timestamp in milliseconds)
        ├── Text Length: uint16
        └── Text: []byte (UTF-8 encoded lyrics text)
```

### Create Custom Adapter

For the ultimate ease of use, the data lrcd sent is mostly in plain text with few exceptions:
- `ETX` (0x03, End-of-Text): Indicates that lrcd is currently inactive
- `EOT` (0x04, End-of-Transmission): Indicates that lrcd has exited

lrcd needs a way to tell the adapter it's state to better integrate into target environment. The special characters above could be safely ignored if not using them.

Once you've decided which way your new adapter receives data, configure the corresponding publisher in lrcd's config file. A simple config would look like this:

```yaml
publishers:
  - id: "file"
    offset: 0
    options:
      path: "/tmp/lrcd.pipe"
      format: "\x1b[H\x1b[J%s"  # Clear terminal
```

Then simply execute the following command:

```bash
cat /tmp/lrcd.pipe
```

And that's it. You've just created a CLI adapter!

> You do not need to handle the special characters defined above as they are literally invisible. Also, `EOT` will act as EOF for `cat`, so the adapter will quit automatically as well if lrcd is terminated.

## Performance

- **Memory Usage**: ~25MB typical
- **CPU Usage**: <1% on modern systems
- **Cache Size**: ~1MB per 1000 cached songs
- **Network**: Minimal, only fetches lyrics for new songs

## Acknowledgments

- [tuberry/desktop-lyric](https://github.com/tuberry/desktop-lyric) for inspiration
- [LRCLIB](https://lrclib.net/) and other providers for the lyrics database
- [CloudFlare](https://github.com/cloudflare) for the Aho-Corasick implementation
- Special Thanks: Rain Cat
