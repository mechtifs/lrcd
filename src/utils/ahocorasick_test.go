package utils

import (
	"strings"
	"testing"
)

const (
	title    = "春日影 (MyGo!!!!! ver.)"
	expected = "春日影"
)

var splitters = []string{"(", "（", "[", "［", "【", "〖", "＜", "〈", "《", "-", "―", "—", " feat.", " ft.", " ver."}

func BenchmarkAhoCorasick(b *testing.B) {
	matcher := NewStringMatcher(splitters)
	var out string
	for b.Loop() {
		b := []byte(title)
		idx := matcher.Index(b)
		if idx == -1 {
			out = title
		} else {
			out = string(b[:idx])
		}
	}
	out = strings.TrimSpace(out)
	b.Log(out)
	if out != expected {
		b.Fail()
	}
}

func BenchmarkMinIndex(b *testing.B) {
	var out string
	for b.Loop() {
		min := len(title)
		for _, sep := range splitters {
			if i := strings.Index(title, sep); i != -1 && i < min {
				min = i
			}
		}
		out = title[:min]
	}
	out = strings.TrimSpace(out)
	b.Log(out)
	if out != expected {
		b.Fail()
	}
}
