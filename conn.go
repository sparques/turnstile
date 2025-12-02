package turnstile

import (
	"io"
	"net"
	"time"
)

type serialAddr string

func (a serialAddr) Network() string { return "serial" }
func (a serialAddr) String() string  { return string(a) }

type rwConn struct {
	io.ReadWriteCloser
	local, remote net.Addr
	onClose       func()
}

func (c *rwConn) LocalAddr() net.Addr              { return c.local }
func (c *rwConn) RemoteAddr() net.Addr             { return c.remote }
func (c *rwConn) SetDeadline(time.Time) error      { return nil }
func (c *rwConn) SetReadDeadline(time.Time) error  { return nil }
func (c *rwConn) SetWriteDeadline(time.Time) error { return nil }
func (c *rwConn) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return c.ReadWriteCloser.Close()
}

type rwNilCloser struct {
	io.ReadWriter
}

func (rwNilCloser) Close() error {
	return nil
}
