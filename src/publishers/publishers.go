package publishers

const (
	FilePublisherID      = "file"
	HTTPPublisherID      = "http"
	WebSocketPublisherID = "websocket"
	DBusPublisherID      = "dbus"
)

type Publisher interface {
	ID() string
	Send(string) error
	Exit() error
}
