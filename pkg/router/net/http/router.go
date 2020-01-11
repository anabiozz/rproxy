package http

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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
	HandleConn(net.Conn)
}

// Matcher ..
type Matcher func(ctx context.Context, hostname string) bool

// Implemets Matcher interface
func equals(want string) Matcher {
	return func(_ context.Context, got string) bool {
		return want == got
	}
}

// Route that impliments route interface
type httpHostMatch struct {
	matcher Matcher
	target  Target
}

func (httphostmatch httpHostMatch) match(br *bufio.Reader) (Target, string) {
	host := httpHostHeader(br)
	if httphostmatch.matcher(context.TODO(), host) {
		return httphostmatch.target, host
	}
	return nil, ""
}

var (
	lfHostColon = []byte("\nHost:")
	lfhostColon = []byte("\nhost:")
	crlf        = []byte("\n")
	lf          = []byte("\n")
	crlfcrlf    = []byte("\n\n")
	lflf        = []byte("\n\n")
)

// return host header value
func httpHostHeader(br *bufio.Reader) string {
	const maxPeek = 4 << 10
	peekSize := 0
	for {
		peekSize++
		if peekSize > maxPeek {
			b, _ := br.Peek(br.Buffered())
			return httpHostHeaderFromBytes(b)
		}
		b, err := br.Peek(peekSize)
		if n := br.Buffered(); n > peekSize {
			b, _ = br.Peek(n)
			peekSize = n
		}
		if len(b) > 0 {
			if b[0] < 'A' || b[0] > 'Z' {
				return ""
			}
			if bytes.Index(b, crlfcrlf) != -1 || bytes.Index(b, lflf) != -1 {
				req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
				if err != nil {
					return ""
				}
				if len(req.Header["Host"]) > 1 {
					return ""
				}
				return req.Host
			}
		}
		if err != nil {
			return httpHostHeaderFromBytes(b)
		}
	}
}

type fixedTarget struct {
	target Target
}

func (m fixedTarget) match(*bufio.Reader) (Target, string) { return m.target, "" }

// Берем значение от Host: до переноса строки
func httpHostHeaderFromBytes(b []byte) string {
	if i := bytes.Index(b, lfHostColon); i != -1 {
		return string(bytes.TrimSpace(untilEOL(b[i+len(lfHostColon):])))
	}
	if i := bytes.Index(b, lfhostColon); i != -1 {
		return string(bytes.TrimSpace(untilEOL(b[i+len(lfhostColon):])))
	}
	return ""
}

func untilEOL(v []byte) []byte {
	if i := bytes.IndexByte(v, '\n'); i != -1 {
		return v[:i]
	}
	return v
}

// Conn ..
type Conn struct {
	HostName string
	Peeked   []byte
	net.Conn
}

func proxyCopy(errc chan<- error, dst, src net.Conn) {

	if wc, ok := src.(*Conn); ok && len(wc.Peeked) > 0 {
		if _, err := dst.Write(wc.Peeked); err != nil {
			errc <- err
			return
		}
		wc.Peeked = nil
	}

	src = UnderlyingConn(src)
	dst = UnderlyingConn(dst)

	_, err := io.Copy(dst, src)
	errc <- err
}

// UnderlyingConn ..
func UnderlyingConn(conn net.Conn) net.Conn {
	if wrap, ok := conn.(*Conn); ok {
		return wrap.Conn
	}
	return conn
}

func goCloseConn(conn net.Conn) { go conn.Close() }

// Close ..
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

// AddHTTPHostRoute ..
func (proxy *Proxy) AddHTTPHostRoute(ipPort, httpHost string, dest Target) {
	proxy.AddHTTPHostMatchRoute(ipPort, equals(httpHost), dest)
}

// AddHTTPHostMatchRoute ..
func (proxy *Proxy) AddHTTPHostMatchRoute(ipPort string, match Matcher, dest Target) {
	proxy.addRoute(ipPort, httpHostMatch{match, dest})
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
func (proxy *Proxy) Start() error {
	if proxy.donec != nil {
		return errors.New("already started")
	}

	proxy.donec = make(chan struct{})
	errc := make(chan error, len(proxy.configs))
	proxy.listeners = make([]net.Listener, 0, len(proxy.configs))
	for ipPort, config := range proxy.configs {
		fmt.Println("ipPort", ipPort)
		listener, err := proxy.netListen()("tcp", ipPort)
		if err != nil {
			proxy.Close()
			return err
		}

		proxy.listeners = append(proxy.listeners, listener)
		go proxy.serveListener(errc, listener, config.routes)
	}
	go proxy.awaitFirstError(errc)
	return nil
}

func (proxy *Proxy) awaitFirstError(errc <-chan error) {
	proxy.err = <-errc
	close(proxy.donec)
}

func (proxy *Proxy) serveListener(errc chan<- error, listener net.Listener, routes []route) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			errc <- err
			return
		}
		go proxy.serveConn(conn, routes)
	}
}

func (proxy *Proxy) serveConn(conn net.Conn, routes []route) bool {

	br := bufio.NewReader(conn)
	for _, route := range routes {
		if target, hostName := route.match(br); target != nil {

			fmt.Println("hostName", hostName)

			if n := br.Buffered(); n > 0 {
				peeked, _ := br.Peek(br.Buffered())
				conn = &Conn{
					HostName: hostName,
					Peeked:   peeked,
					Conn:     conn,
				}
			}

			target.HandleConn(conn)
			return true
		}
	}

	fmt.Printf("no routes matched conn %v/%v; closing\n", conn.RemoteAddr().String(), conn.LocalAddr().String())
	conn.Close()
	return false
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

// GenerateProxy ..
// func GenerateProxy(ctx context.Context, conf config.ProviderConfiguration) http.Handler {

// 	ctxLog := log.NewContext(ctx, log.Str("router", conf.Name))
// 	logger := log.WithContext(ctxLog)

// 	proxy := &httputil.ReverseProxy{Director: func(req *http.Request) {

// 		// requestParam := req.URL.Query().Get("proxy")

// 		target, err := url.Parse(conf.Host)
// 		if err != nil {
// 			logger.Error(err)
// 			return
// 		}

// 		req.URL.Host = target.Host
// 		req.URL.Scheme = target.Scheme
// 		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
// 		req.Header.Set("X-Origin-Host", target.Host)
// 		req.Host = target.Host

// 	}, Transport: &http.Transport{
// 		Dial: (&net.Dialer{
// 			Timeout: 5 * time.Second,
// 		}).Dial,
// 	},
// 	}

// 	logger.Info("Router was UP")

// 	return proxy
// }

// // GenerateProxy ..
// func GenerateProxy(ctx context.Context, services map[string]config.Service, providerName string) http.Handler {
// 	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

// 		ctxLog := log.NewContext(ctx, log.Str("router", providerName))
// 		logger := log.WithContext(ctxLog)

// 		requestParam := req.URL.Query().Get("proxy")
// 		service := services[requestParam]

// 		target, err := url.Parse(service.Host)
// 		if err != nil {
// 			logger.Error(err)
// 			return
// 		}

// 		proxy := httputil.NewSingleHostReverseProxy(target)
// 		req.URL.Host = target.Host
// 		req.URL.Scheme = target.Scheme
// 		req.Header.Set("X-Forwarded-Host", req.Host)
// 		req.Header.Set("X-Origin-Host", target.Host)
// 		req.Host = target.Host

// 		proxy.Transport = &rProxyTransport{
// 			logger: logger,
// 			Transport: &http.Transport{
// 				Proxy: http.ProxyFromEnvironment,
// 				// Dial: func(network, addr string) (net.Conn, error) {
// 				// 	addr = strings.Split(addr, ":")[0]
// 				// 	tmp := strings.Split(addr, "/")
// 				// 	if len(tmp) != 2 {
// 				// 		return nil, ErrInvalidService
// 				// 	}
// 				// 	return loadBalance(network, tmp[0], tmp[1])
// 				// },
// 				TLSHandshakeTimeout: 10 * time.Second,
// 			},
// 		}

// 		proxy.ServeHTTP(res, req)
// 	})
// }
