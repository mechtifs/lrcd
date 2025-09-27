package utils

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	"lrcd/models"
)

func Equal(a *models.MPRISMetadata, b *models.MPRISMetadata) bool {
	return a.Title == b.Title && slices.Equal(a.Artists, b.Artists)
}

func FormatTrack(meta *models.MPRISMetadata) string {
	if meta.Title == "" {
		return "<nil>"
	}
	artists := slices.Clone(meta.Artists)
	slices.Sort(artists)
	builder := &strings.Builder{}
	builder.WriteString(meta.Title)
	builder.WriteString(" -")
	for _, artist := range artists {
		builder.WriteByte(' ')
		builder.WriteString(artist)
	}
	return builder.String()
}

func FormatFilename(meta *models.MPRISMetadata) string {
	return strings.ReplaceAll(FormatTrack(meta), "/", "_") + ".cache"
}

func ParseLrc(lrc string) ([]*models.LyricLine, error) {
	data := []byte(lrc)
	lines := []*models.LyricLine{}
	for len(data) > 0 {
		var line []byte
		lineLen := bytes.IndexByte(data, '\n')
		if lineLen == 0 {
			data = data[1:]
			continue
		} else if lineLen == -1 {
			line = data
			data = nil
		} else {
			line = data[:lineLen]
			data = data[lineLen+1:]
		}

		postitions := []int{}
		for len(line) > 0 && line[0] == '[' {
			j := bytes.IndexByte(line, ']')
			if j == -1 {
				break
			}
			p, ok := parseLRCPosition(line[1:j])
			if !ok {
				break
			}
			postitions = append(postitions, p)
			line = line[j+1:]
		}
		text := string(bytes.TrimSpace(line))
		for _, t := range postitions {
			lines = append(lines, &models.LyricLine{Position: t, Text: text})
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("lrc not synced")
	}
	slices.SortFunc(lines, func(a, b *models.LyricLine) int { return a.Position - b.Position })
	return lines, nil
}

func parseLRCPosition(s []byte) (int, bool) {
	sLen := len(s)
	if sLen < 5 || sLen > 12 {
		return 0, false
	}

	n := 0
	sec := 0
	sepIdx := 0
	sepCnt := 0
	for i, ch := range []byte(s) {
		if ch == ':' || ch == '.' {
			sec = sec*60 + n
			n = 0
			sepIdx = i
			sepCnt++
			continue
		}
		ch -= '0'
		if ch > 9 {
			return 0, false
		}
		n = n*10 + int(ch)
	}

	if sLen-sepIdx == 3 && sepCnt == 1 && s[sepIdx] == ':' { // Handle `[mm:ss]`
		return (sec*60 + n) * 1000, true
	}

	position := sec * 1000
	switch sLen - sepIdx {
	case 2:
		position += n * 100
	case 3:
		position += n * 10
	case 4:
		position += n
	}

	return position, true
}

var matcher = NewStringMatcher([]string{"(", "（", "[", "［", "【", "〖", "＜", "〈", "《", "-", "―", "—", " feat.", " ft.", " ver."})

func StripTitle(title string) string {
	b := []byte(title)
	idx := matcher.Index(b)
	if idx == -1 {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(string(b[:idx]))
}
