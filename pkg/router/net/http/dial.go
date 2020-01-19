package http

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

// DialProxy is target
type DialProxy struct {
	Addr            string
	KeepAlivePeriod time.Duration
	DialTimeout     time.Duration
	DialContext     func(ctx context.Context, network, address string) (net.Conn, error)
	OnDialError     func(src net.Conn, dstDialErr error)
}

var defaultDialer = new(net.Dialer)

func (dialproxy *DialProxy) dialContext() func(ctx context.Context, network, address string) (net.Conn, error) {
	if dialproxy.DialContext != nil {
		return dialproxy.DialContext
	}
	return defaultDialer.DialContext
}

func (dialproxy *DialProxy) onDialError() func(src net.Conn, dstDialErr error) {
	if dialproxy.OnDialError != nil {
		return dialproxy.OnDialError
	}
	return func(src net.Conn, dstDialErr error) {
		fmt.Printf("for incoming conn %v, error dialing %q: %v\n", src.RemoteAddr().String(), dialproxy.Addr, dstDialErr)
		src.Close()
	}
}

func (dialproxy *DialProxy) keepAlivePeriod() time.Duration {
	if dialproxy.KeepAlivePeriod != 0 {
		return dialproxy.KeepAlivePeriod
	}
	return 1 * time.Minute
}

func goCloseConn(conn net.Conn) {
	go conn.Close()
}

// HandleConn ..
func (dialproxy *DialProxy) HandleConn(ctx context.Context, src net.Conn) {

	var cancel context.CancelFunc
	if dialproxy.DialTimeout >= 0 {
		ctx, cancel = context.WithTimeout(ctx, dialproxy.dialTimeout())
	}

	dst, err := dialproxy.dialContext()(ctx, "tcp", dialproxy.Addr)
	if cancel != nil {
		cancel()
	}

	if err != nil {
		dialproxy.onDialError()(src, err)
		return
	}

	defer goCloseConn(dst)
	defer goCloseConn(src)

	if ka := dialproxy.keepAlivePeriod(); ka > 0 {
		if c, ok := UnderlyingConn(src).(*net.TCPConn); ok {
			c.SetKeepAlive(true)
			c.SetKeepAlivePeriod(ka)
		}
		if c, ok := dst.(*net.TCPConn); ok {
			c.SetKeepAlive(true)
			c.SetKeepAlivePeriod(ka)
		}
	}

	errc := make(chan error, 1)
	go proxyCopy(errc, src, dst)
	go proxyCopy(errc, dst, src)

	<-errc
}

func proxyCopy(errc chan<- error, dst, src net.Conn) {

	if srcconn, ok := src.(*Conn); ok && len(srcconn.Peeked) > 0 {
		if _, err := dst.Write(srcconn.Peeked); err != nil {
			errc <- err
			return
		}
		srcconn.Peeked = nil
	}

	src = UnderlyingConn(src)
	dst = UnderlyingConn(dst)

	_, err := io.Copy(dst, src)
	errc <- err
}

func (dialproxy *DialProxy) dialTimeout() time.Duration {
	if dialproxy.DialTimeout > 0 {
		return dialproxy.DialTimeout
	}
	return 10 * time.Second
}

// To ..
func To(addr string) *DialProxy {
	return &DialProxy{Addr: addr}
}
