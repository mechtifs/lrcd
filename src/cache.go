package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"

	"lrcd/models"
	"lrcd/utils"

	"github.com/pierrec/lz4/v4"
)

type Cache struct {
	path string
}

type CacheHeader struct {
	Signature [4]byte
	BodySize  uint32
	LineCount uint16
	Source    [6]byte
}

var (
	signature            = [4]byte{'l', 'r', 'c', 'd'}
	ErrSignatureMismatch = errors.New("signature mismatch")
)

func (c *Cache) Set(meta *models.MPRISMetadata, lyrics *models.Lyrics) error {
	bodySize := 0
	for _, line := range lyrics.Lines {
		bodySize += len(line.Text) + 6
	}
	h := CacheHeader{
		Signature: signature,
		LineCount: uint16(lyrics.Len()),
		BodySize:  uint32(bodySize),
	}
	copy(h.Source[:], []byte(lyrics.Source))
	header := make([]byte, binary.Size(h))
	binary.Encode(header, binary.LittleEndian, h)
	body := make([]byte, bodySize)
	offset := 0
	for _, line := range lyrics.Lines {
		textLen := len(line.Text)
		binary.LittleEndian.PutUint32(body[offset:], uint32(line.Position))
		offset += 4
		binary.LittleEndian.PutUint16(body[offset:], uint16(textLen))
		offset += 2
		copy(body[offset:], []byte(line.Text))
		offset += textLen
	}
	compressed := make([]byte, lz4.CompressBlockBound(bodySize))
	compressor := lz4.CompressorHC{Level: lz4.Level9}
	n, err := compressor.CompressBlock(body, compressed)
	if err != nil {
		return err
	}
	fpath := filepath.Join(c.path, utils.FormatFilename(meta))
	tmp := fpath + ".tmp"
	err = os.WriteFile(tmp, append(header, compressed[:n]...), 0o644)
	if err != nil {
		return err
	}
	return os.Rename(tmp, fpath)
}

func (c *Cache) Get(meta *models.MPRISMetadata) (*models.Lyrics, error) {
	f, err := os.Open(filepath.Join(c.path, utils.FormatFilename(meta)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	header := CacheHeader{}
	err = binary.Read(f, binary.LittleEndian, &header)
	if err != nil {
		return nil, err
	}
	if header.Signature != signature {
		return nil, ErrSignatureMismatch
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	deflated := make([]byte, header.BodySize)
	_, err = lz4.UncompressBlock(buf, deflated)
	if err != nil {
		return nil, err
	}
	lines := make([]*models.LyricLine, header.LineCount)
	offset := 0
	for i := range int(header.LineCount) {
		position := int(binary.LittleEndian.Uint32(deflated[offset:]))
		offset += 4
		textLen := int(binary.LittleEndian.Uint16(deflated[offset:]))
		offset += 2
		lines[i] = &models.LyricLine{
			Position: position,
			Text:     string(deflated[offset : offset+textLen]),
		}
		offset += textLen
	}
	for offset = 5; offset >= 0; offset-- {
		if header.Source[offset] != '\x00' {
			break
		}
	}
	return &models.Lyrics{
		Lines:  lines,
		Source: string(header.Source[:offset+1]),
	}, nil
}
