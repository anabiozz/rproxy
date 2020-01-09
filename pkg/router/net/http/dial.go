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
	Addr                 string
	KeepAlivePeriod      time.Duration
	DialTimeout          time.Duration
	DialContext          func(ctx context.Context, network, address string) (net.Conn, error)
	OnDialError          func(src net.Conn, dstDialErr error)
	ProxyProtocolVersion int
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
	return time.Minute
}

func (dialproxy *DialProxy) sendProxyHeader(w io.Writer, src net.Conn) error {
	switch dialproxy.ProxyProtocolVersion {
	case 0:
		return nil
	case 1:
		var srcAddr, dstAddr *net.TCPAddr
		if a, ok := src.RemoteAddr().(*net.TCPAddr); ok {
			srcAddr = a
		}
		if a, ok := src.LocalAddr().(*net.TCPAddr); ok {
			dstAddr = a
		}

		if srcAddr == nil || dstAddr == nil {
			_, err := io.WriteString(w, "PROXY UNKNOWN\r\n")
			return err
		}

		family := "TCP4"
		if srcAddr.IP.To4() == nil {
			family = "TCP6"
		}
		_, err := fmt.Fprintf(w, "PROXY %s %s %d %s %d\r\n", family, srcAddr.IP, srcAddr.Port, dstAddr.IP, dstAddr.Port)
		return err
	default:
		return fmt.Errorf("proxy protocol version %d not supported", dialproxy.ProxyProtocolVersion)
	}
}

// HandleConn ..
func (dialproxy *DialProxy) HandleConn(src net.Conn) {

	ctx := context.Background()

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

	if err = dialproxy.sendProxyHeader(dst, src); err != nil {
		dialproxy.onDialError()(src, err)
		return
	}

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

func (dialproxy *DialProxy) dialTimeout() time.Duration {
	if dialproxy.DialTimeout > 0 {
		return dialproxy.DialTimeout
	}
	return 1 * time.Second
}

// To ..
func To(addr string) *DialProxy {
	return &DialProxy{Addr: addr}
}
