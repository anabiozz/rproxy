package http

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http/httptrace"
	"time"

	"github.com/anabiozz/rproxy/pkg/log"
)

// Proxy ..
type Proxy struct {
	configs    map[string]*routerConfig
	listeners  []net.Listener
	donec      chan struct{}
	err        error
	ListenFunc func(net, laddr string) (net.Listener, error)
}

type routerConfig struct {
	routes []route
}

type route interface {
	match(*bufio.Reader) (Target, string)
}

// Target ..
type Target interface {
	HandleConn(context.Context, net.Conn)
}

// Matcher ..
type Matcher func(ctx context.Context, hostname string) bool

type fixedTarget struct {
	target Target
}

func (m fixedTarget) match(*bufio.Reader) (Target, string) {
	return m.target, ""
}

// Conn ..
type Conn struct {
	HostName string
	Peeked   []byte
	net.Conn
}

// UnderlyingConn returns underlying connection
func UnderlyingConn(conn net.Conn) net.Conn {
	if uc, ok := conn.(*Conn); ok {
		return uc.Conn
	}
	return conn
}

// Close closes all listeners
func (proxy *Proxy) Close() error {
	for _, c := range proxy.listeners {
		c.Close()
	}
	return nil
}

// AddRoute ..
func (proxy *Proxy) AddRoute(ipPort string, dest Target) {
	proxy.addRoute(ipPort, fixedTarget{dest})
}

func (proxy *Proxy) addRoute(ipPort string, route route) {
	cfg := proxy.configFor(ipPort)
	cfg.routes = append(cfg.routes, route)
}

func (proxy *Proxy) configFor(ipPort string) *routerConfig {
	if proxy.configs == nil {
		proxy.configs = make(map[string]*routerConfig)
	}
	if proxy.configs[ipPort] == nil {
		proxy.configs[ipPort] = &routerConfig{}
	}
	return proxy.configs[ipPort]
}

// Start ..
func (proxy *Proxy) Start(ctx context.Context) error {
	if proxy.donec != nil {
		return errors.New("already started")
	}

	proxy.donec = make(chan struct{})
	errc := make(chan error, len(proxy.configs))
	proxy.listeners = make([]net.Listener, 0, len(proxy.configs))

	for ipPort, config := range proxy.configs {

		listener, err := proxy.netListen()("tcp", ipPort)
		if err != nil {
			proxy.Close()
			return err
		}

		proxy.listeners = append(proxy.listeners, listener)

		go proxy.serveListener(ctx, errc, listener, config.routes)
	}
	go proxy.awaitFirstError(errc)
	return nil
}

func (proxy *Proxy) awaitFirstError(errc <-chan error) {
	proxy.err = <-errc
	close(proxy.donec)
}

func (proxy *Proxy) serveListener(ctx context.Context, errc chan<- error, listener net.Listener, routes []route) {

	ctxLog := log.NewContext(ctx, log.Str("function", "serveListener"))
	logger := log.WithContext(ctxLog)

	var start, connect, dns, tlsHandshake time.Time

	start = time.Now()

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			logger.Infof("DNS DONE: %v\n", time.Since(dns))
		},

		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			logger.Infof("TLS HANDSHAKE: %v\n", time.Since(tlsHandshake))
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			logger.Infof("CONNECT TIME: %v\n", time.Since(connect))
		},

		GotFirstResponseByte: func() {
			logger.Infof("TIME FROM START TO FIRST BYTE: %v\n", time.Since(start))
		},

		WroteHeaderField: func(key string, value []string) {
			logger.Infof("WROTE HEADER FIELDS: %s, %v\n", key, value)
		},

		WroteHeaders: func() {
			logger.Infof("WROTE HEADERS: %v\n", time.Since(connect))
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			logger.Infof("WROTE REQUEST INFO: %v\n", info)
		},
	}

	ctx = httptrace.WithClientTrace(ctx, trace)

	for {
		conn, err := listener.Accept()
		if err != nil {
			errc <- err
			return
		}

		go proxy.serveConn(ctx, conn, routes)
	}
}

func (proxy *Proxy) serveConn(ctx context.Context, conn net.Conn, routes []route) {

	bufreader := bufio.NewReader(conn)

	for _, route := range routes {
		if target, hostName := route.match(bufreader); target != nil {

			if n := bufreader.Buffered(); n > 0 {
				peeked, err := bufreader.Peek(bufreader.Buffered())
				if err != nil {
					fmt.Println(err)
				}

				conn = &Conn{
					HostName: hostName,
					Peeked:   peeked,
					Conn:     conn,
				}
			}

			target.HandleConn(ctx, conn)
			return
		}
	}

	fmt.Printf("no routes matched conn %v/%v; closing\n", conn.RemoteAddr().String(), conn.LocalAddr().String())
	conn.Close()
	return
}

// if ListenFunc not chosen, net.Listen will be return
func (proxy *Proxy) netListen() func(net, laddr string) (net.Listener, error) {
	if proxy.ListenFunc != nil {
		return proxy.ListenFunc
	}
	return net.Listen
}

// Common errors
var (
	ErrInvalidService = errors.New("invalid service/version")
)

func loadBalance(network, serviceName, serviceVersion string) (net.Conn, error) {
	return nil, nil
}
