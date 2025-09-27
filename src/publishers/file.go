package publishers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type FilePublisher struct {
	fd     *os.File
	format string
}

type FilePublisherOptions struct {
	Path   string
	Format string
}

func NewFilePublisher(opt *FilePublisherOptions) *FilePublisher {
	var fd *os.File
	if !filepath.IsAbs(opt.Path) {
		panic("file path must be absolute")
	}
	path := filepath.Clean(opt.Path)
	stat, err := os.Stat(opt.Path)
	if err == nil {
		if stat.Mode().Type() == os.ModeNamedPipe {
			fd, err = os.OpenFile(path, os.O_RDWR, os.ModeNamedPipe)
		} else {
			fd, err = os.OpenFile(path, os.O_WRONLY, 0644)
		}
	} else {
		if strings.HasSuffix(path, ".pipe") {
			err = syscall.Mkfifo(path, 0644)
			if err != nil {
				panic(err)
			}
			fd, err = os.OpenFile(path, os.O_RDWR, os.ModeNamedPipe)
		} else {
			fd, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
		}
	}
	if err != nil {
		panic(err)
	}
	return &FilePublisher{
		fd:     fd,
		format: opt.Format,
	}
}

func (*FilePublisher) ID() string {
	return FilePublisherID
}

func (p *FilePublisher) Send(txt string) error {
	_, err := fmt.Fprintf(p.fd, p.format, txt)
	return err
}

func (p *FilePublisher) Exit() error {
	return p.fd.Close()
}
