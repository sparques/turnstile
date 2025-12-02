package turnstile

import (
	"context"
	"io"
	"net"
	"sync"
	"time"
)

// OpenFunc, serialAddr, rwConn, rwNilCloser, and reopenListener definitions
// are exactly as in your existing code.

// --- Client-side: one-at-a-time dialer over an io.ReadWriteCloser ---

type reopenDialer struct {
	open OpenFunc
	addr net.Addr

	mu       sync.Mutex
	closed   bool
	closedCh chan struct{} // non-nil while a conn is active; closed when that conn closes
}

func NewReopenDialer(open OpenFunc, name string) *reopenDialer {
	return &reopenDialer{
		open: open,
		addr: serialAddr(name),
	}
}

func NewReadWriterDialer(rw io.ReadWriter, name string) *reopenDialer {
	return &reopenDialer{
		open: func() (io.ReadWriteCloser, error) {
			return rwNilCloser{rw}, nil
		},
		addr: serialAddr(name),
	}
}

// Close prevents future Dial calls from succeeding and wakes any blocked callers.
func (d *reopenDialer) Close() error {
	d.mu.Lock()
	d.closed = true
	ch := d.closedCh
	d.mu.Unlock()

	if ch != nil {
		// Wake any Dial blocked waiting for the current conn to finish.
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	return nil
}

// DialContext returns a single active net.Conn at a time, blocking until
// the previous conn (if any) is closed, or until ctx is cancelled.
func (d *reopenDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// Fast-fail if context already cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Ensure only one active connection at a time.
	for {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return nil, net.ErrClosed
		}
		// If a connection is already active, wait for it to close.
		if ch := d.closedCh; ch != nil {
			d.mu.Unlock()
			select {
			case <-ch:
				// Try again; the conn is now closed.
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		// No active conn; we're the one that gets to open it.
		d.mu.Unlock()
		break
	}

	// Retry loop to open the underlying RWC with backoff.
	backoff := 100 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		c, err := d.open()
		if err == nil {
			d.mu.Lock()
			if d.closed {
				d.mu.Unlock()
				c.Close()
				return nil, net.ErrClosed
			}
			ch := make(chan struct{})
			d.closedCh = ch
			d.mu.Unlock()

			rc := &rwConn{
				ReadWriteCloser: c,
				local:           d.addr,
				// The "remote" here is largely cosmetic; HTTP clients don't care.
				remote: serialAddr(address),
				onClose: func() {
					d.mu.Lock()
					if d.closedCh != nil {
						close(d.closedCh)
						d.closedCh = nil
					}
					d.mu.Unlock()
				},
			}
			return rc, nil
		}

		// If the dialer has been closed, stop retrying.
		d.mu.Lock()
		closed := d.closed
		d.mu.Unlock()
		if closed {
			return nil, net.ErrClosed
		}

		// Backoff, but remain cancellable by ctx.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

// Dial is a convenience wrapper for DialContext with a background context.
// This makes it plug in nicely anywhere a plain Dial func is accepted.
func (d *reopenDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}
