package publishers

import (
	"github.com/godbus/dbus/v5"
)

type DBusPublisher struct {
	conn *dbus.Conn
	path dbus.ObjectPath
	name string
}

type DBusPublisherOptions struct {
	Path string
	Name string
}

func NewDBusPublisher(opt *DBusPublisherOptions) *DBusPublisher {
	conn, _ := dbus.ConnectSessionBus() // Since DBUS_SESSION_BUS_ADDRESS is already set, there shouldn't be any error
	return &DBusPublisher{
		conn: conn,
		path: dbus.ObjectPath(opt.Path),
		name: opt.Name,
	}
}

func (*DBusPublisher) ID() string {
	return DBusPublisherID
}

func (p *DBusPublisher) Send(txt string) error {
	return p.conn.Emit(p.path, p.name, txt)
}

func (*DBusPublisher) Exit() error {
	return nil // This is managed in the main goroutine
}
