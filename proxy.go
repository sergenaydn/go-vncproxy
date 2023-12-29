package vncproxy

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TokenHandler represents a function to extract the VNC backend address from an HTTP request
type TokenHandler func(r *http.Request) (addr string, err error)

// Config represents VNC proxy config
type Config struct {
	LogLevel    uint32
	Logger      Logger
	DialTimeout time.Duration
	TokenHandler
}

// Proxy represents VNC proxy
type Proxy struct {
	logLevel     uint32
	logger       *logger
	dialTimeout  time.Duration
	peers        map[*peer]struct{}
	l            sync.RWMutex
	tokenHandler TokenHandler
}

// New returns a VNC proxy
// If the token handler is nil, the VNC backend address will always be :5901
func New(conf *Config) *Proxy {
	if conf.TokenHandler == nil {
		conf.TokenHandler = func(r *http.Request) (addr string, err error) {
			return ":5901", nil
		}
	}

	return &Proxy{
		logLevel:     conf.LogLevel,
		logger:       NewLogger(conf.LogLevel, conf.Logger),
		dialTimeout:  conf.DialTimeout,
		peers:        make(map[*peer]struct{}),
		l:            sync.RWMutex{},
		tokenHandler: conf.TokenHandler,
	}
}

// ServeWS provides WebSocket handler
func (p *Proxy) ServeWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Info("Upgrade to WebSocket failed:", err)
		return
	}

	p.logger.Debugf("ServeWS")
	p.logger.Debugf("request url: %v", r.URL)

	// get VNC backend server address
	addr, err := p.tokenHandler(r)
	if err != nil {
		p.logger.Infof("get VNC backend failed: %v", err)
		conn.Close()
		return
	}

	peer, err := NewPeer(conn, addr, p.dialTimeout)
	if err != nil {
		p.logger.Infof("new VNC peer failed: %v", err)
		conn.Close()
		return
	}

	p.addPeer(peer)
	defer func() {
		p.logger.Info("close peer")
		p.deletePeer(peer)
	}()

	go func() {
		if err := peer.ReadTarget(); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			p.logger.Info(err)
			return
		}
	}()

	if err = peer.ReadSource(); err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return
		}
		p.logger.Info(err)
		return
	}
}

func (p *Proxy) addPeer(peer *peer) {
	p.l.Lock()
	p.peers[peer] = struct{}{}
	p.l.Unlock()
}

func (p *Proxy) deletePeer(peer *peer) {
	p.l.Lock()
	delete(p.peers, peer)
	peer.Close()
	p.l.Unlock()
}

func (p *Proxy) Peers() map[*peer]struct{} {
	p.l.RLock()
	defer p.l.RUnlock()
	return p.peers
}
