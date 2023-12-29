package vncproxy

import (
	"net"
	"time"

	"github.com/evangwt/go-bufcopy"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	defaultDialTimeout = 5 * time.Second
)

var (
	bcopy = bufcopy.New()
)

// peer represents a VNC proxy peer
// with a WebSocket connection and a VNC backend connection
type peer struct {
	source *websocket.Conn
	target net.Conn
}

// NewPeer creates a new VNC proxy peer
func NewPeer(ws *websocket.Conn, addr string, dialTimeout time.Duration) (*peer, error) {
	if ws == nil {
		return nil, errors.New("WebSocket connection is nil")
	}

	if len(addr) == 0 {
		return nil, errors.New("addr is empty")
	}

	if dialTimeout <= 0 {
		dialTimeout = defaultDialTimeout
	}
	c, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to VNC backend")
	}

	err = c.(*net.TCPConn).SetKeepAlive(true)
	if err != nil {
		return nil, errors.Wrap(err, "enable VNC backend connection keepalive failed")
	}

	err = c.(*net.TCPConn).SetKeepAlivePeriod(30 * time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "set VNC backend connection keepalive period failed")
	}

	return &peer{
		source: ws,
		target: c,
	}, nil
}

// ReadSource copies source stream to target connection
func (p *peer) ReadSource() error {
	if _, err := bcopy.Copy(p.target, websocketReader{p.source}); err != nil {
		return errors.Wrapf(err, "copy source(%v) => target(%v) failed", p.source.RemoteAddr(), p.target.RemoteAddr())
	}
	return nil
}

// ReadTarget copies target stream to source connection
func (p *peer) ReadTarget() error {
	if _, err := bcopy.Copy(websocketWriter{p.source}, p.target); err != nil {
		return errors.Wrapf(err, "copy target(%v) => source(%v) failed", p.target.RemoteAddr(), p.source.RemoteAddr())
	}
	return nil
}

// Close closes the WebSocket connection and the VNC backend connection
func (p *peer) Close() {
	p.source.Close()
	p.target.Close()
}

// Helper structs to adapt WebSocket connection for io.Reader and io.Writer
type websocketReader struct {
	conn *websocket.Conn
}

func (r websocketReader) Read(p []byte) (n int, err error) {
	_, data, err := r.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	n = copy(p, data)
	return n, nil
}

type websocketWriter struct {
	conn *websocket.Conn
}

func (w websocketWriter) Write(p []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
