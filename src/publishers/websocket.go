package publishers

import (
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocketPublisherClient struct {
	send chan string
	conn *websocket.Conn
}

type WebSocketPublisher struct {
	broadcast chan string
	mu        sync.Mutex
	clients   map[*WebSocketPublisherClient]struct{}
	txt       string
	server    *http.Server
}

type WebSocketPublisherOptions struct {
	Address string
}

func NewWebSocketPublisher(opt *WebSocketPublisherOptions) *WebSocketPublisher {
	p := &WebSocketPublisher{
		broadcast: make(chan string, 1),
		clients:   make(map[*WebSocketPublisherClient]struct{}),
	}

	go func() {
		for txt := range p.broadcast {
			p.mu.Lock()
			for c := range p.clients {
				select {
				case c.send <- txt:
				default:
				}
			}
			p.mu.Unlock()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", p.indexFunc)
	p.server = &http.Server{
		Addr:    opt.Address,
		Handler: mux,
	}
	go func() {
		err := p.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()
	return p
}

func (*WebSocketPublisher) ID() string {
	return WebSocketPublisherID
}

func (p *WebSocketPublisher) indexFunc(w http.ResponseWriter, r *http.Request) {
	upgrader := &websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
		Error:       func(http.ResponseWriter, *http.Request, int, error) {},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.Write([]byte(p.txt))
		return
	}

	c := &WebSocketPublisherClient{
		send: make(chan string, 1),
		conn: conn,
	}

	p.mu.Lock()
	p.clients[c] = struct{}{}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.clients, c)
		p.mu.Unlock()
		conn.Close()
	}()

	err = conn.WriteMessage(websocket.TextMessage, []byte(p.txt))
	if err != nil {
		return
	}
	for txt := range c.send {
		err = conn.WriteMessage(websocket.TextMessage, []byte(txt))
		if err != nil {
			return
		}
	}
}

func (p *WebSocketPublisher) Send(txt string) error {
	p.txt = txt
	p.broadcast <- txt
	return nil
}

func (p *WebSocketPublisher) Exit() error {
	return p.server.Close()
}
