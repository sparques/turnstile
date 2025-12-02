package turnstile

import (
	"io"
	"net"
	"time"
)

// serialAddr implements net.Addr
// Network() always returns "serial"
// String() returns serialAddr itself (cast to a string).
type serialAddr string

func (a serialAddr) Network() string { return "serial" }
func (a serialAddr) String() string  { return string(a) }

// rwConn implements net.Conn
// net.Conn is an interface that includes an io.ReadWriteCloser()
// so to use an io.ReadWriterCloser as a net.Conn, only the remaining
// methods of net.Conn need to be implemented.
//
// All the Deadline methods (SetDeadline, SetReadDeadline,
// SetWriteDeadline) are nil operations.
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

// rwNilCloser is a small utility type. It has a nil-operation
// Close() method so that an io.ReadWriter can be used as
// an io.ReadWriteCloser.
type rwNilCloser struct {
	io.ReadWriter
}

func (rwNilCloser) Close() error {
	return nil
}
