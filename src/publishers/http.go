package publishers

import (
	"bytes"
	"net/http"
)

type HTTPPublisher struct {
	method string
	url    string
}

type HTTPPublisherOptions struct {
	Method string
	URL    string
}

func NewHTTPPublisher(opt *HTTPPublisherOptions) *HTTPPublisher {
	return &HTTPPublisher{
		method: opt.Method,
		url:    opt.URL,
	}
}

func (*HTTPPublisher) ID() string {
	return HTTPPublisherID
}

func (p *HTTPPublisher) Send(txt string) error {
	r, _ := http.NewRequest(p.method, p.url, bytes.NewBuffer([]byte(txt)))
	_, err := http.DefaultClient.Do(r)
	return err
}

func (*HTTPPublisher) Exit() error {
	return nil
}
