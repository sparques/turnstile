package turnstile

import (
	"io"
	"net"
	"sync"
	"time"
)

// Adapt an io.ReadWriteCloser (e.g., your serial/pipe) into a net.Listener that:
//  - returns exactly one active net.Conn at a time
//  - blocks Accept() until that conn is closed
//  - re-opens on the next Accept() after close, with optional backoff

type OpenFunc func() (io.ReadWriteCloser, error)

type reopenListener struct {
	open   OpenFunc
	addr   net.Addr
	mu     sync.Mutex
	closed bool
	// closedCh is non-nil while a connection is active; it is closed when that conn closes.
	closedCh chan struct{}
}

func NewReopenListener(open OpenFunc, name string) net.Listener {
	return &reopenListener{
		open: open,
		addr: serialAddr(name),
	}
}

func NewReadWriterListener(rw io.ReadWriter, name string) net.Listener {
	return &reopenListener{
		open: func() (io.ReadWriteCloser, error) {
			return rwNilCloser{rw}, nil
		},
		addr: serialAddr(name),
	}
}

func (l *reopenListener) Addr() net.Addr { return l.addr }

func (l *reopenListener) Close() error {
	l.mu.Lock()
	l.closed = true
	ch := l.closedCh
	l.mu.Unlock()
	// Wake any Accept() blocked on current connection finishing.
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	return nil
}

func (l *reopenListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, net.ErrClosed
	}
	// If a connection is already active, wait for it to close.
	if ch := l.closedCh; ch != nil {
		l.mu.Unlock()
		<-ch
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return nil, net.ErrClosed
		}
	}
	// No active conn; we become the one to open it.
	l.mu.Unlock()

	// Retry loop to open the underlying port with backoff.
	backoff := 100 * time.Millisecond
	for {
		if c, err := l.open(); err == nil {
			l.mu.Lock()
			if l.closed {
				l.mu.Unlock()
				c.Close()
				return nil, net.ErrClosed
			}
			ch := make(chan struct{})
			l.closedCh = ch
			l.mu.Unlock()

			rc := &rwConn{
				ReadWriteCloser: c,
				local:           l.addr,
				remote:          serialAddr("peer"),
				onClose: func() {
					l.mu.Lock()
					if l.closedCh != nil {
						close(l.closedCh)
						l.closedCh = nil
					}
					l.mu.Unlock()
				},
			}
			return rc, nil
		} else {
			if l.closed {
				return nil, net.ErrClosed
			}
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
			}
		}
	}
}
