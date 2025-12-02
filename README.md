# Turnstile

Turnstile is a go package for using an io.ReadWriter or io.ReadWriteCloser as a net.Conn.

This allows, for example, using a serial line to serve HTTP. Serving HTTP over the serial line acts like a single HTTP connection with an infinite KeepAlive.

If your serial connection is unreliable, consider layering this with an [HDLC package](https://github.com/sparques/hdlc).

# Examples
## "Listen" for a Connection (server-side)

```go
// NB: to set baudrate on a serial device, you need a proper serial
// package. e.g. go.bug.st/serial

openSerial := func() (io.ReadWriteCloser, error) {
	return os.OpenFile("/dev/ttyUSB",os.O_RDWR))
}

l := turnstile.NewReopenListener(func() (io.ReadWriteCloser, error) {
	return os.OpenFile("/dev/ttyUSB",os.O_RDWR))
}, "/dev/ttyUSB0")

srv := &http.Server{
	Handler: h,
	// Connection never really goes away, so tell HTTP to "keep open" the connection
	// so we don't waste cycles on nil operations.
	DisableKeepAlives: false,
}

srv.Serve(conn) // blocking
```

## "Opening" a Connection (client-side)

Here's an example of using turnstile with go's HTTP client.


```go
openSerial := func() (io.ReadWriteCloser, error) {
	return os.OpenFile("/dev/ttyUSB",os.O_RDWR))
}

dialer := turnstile.NewReopenDialer(openSerial, "/dev/ttyUSB0")

tr := &http.Transport{
    DialContext: dialer.DialContext,
}

client := &http.Client{Transport: tr}

```

# Why "turnstile"?

A physical turnstile takes what would otherwise be a willy-nilly free for all of human traffic into a one-at-a-time, mediated gateway. 

  - Exactly one thing passes at a time
  - A turnstile adds a line discipline mechanism

That is what this package does.