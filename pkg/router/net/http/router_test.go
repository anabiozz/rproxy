package http

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"
)

type noopTarget struct{}

func (noopTarget) HandleConn(net.Conn) {}

func TestMatchHTTPHost(t *testing.T) {
	tests := []struct {
		name string
		r    io.Reader
		host string
		want bool
	}{
		{
			name: "match",
			r:    strings.NewReader("GET / HTTP/1.1\r\nHost: foo.com\r\n\r\n"),
			host: "foo.com",
			want: true,
		},
		{
			name: "no-match",
			r:    strings.NewReader("GET / HTTP/1.1\r\nHost: foo.com\r\n\r\n"),
			host: "bar.com",
			want: false,
		},
		{
			name: "match-huge-request",
			r:    io.MultiReader(strings.NewReader("GET / HTTP/1.1\r\nHost: foo.com\r\n"), neverEnding('a')),
			host: "foo.com",
			want: true,
		},
	}
	for i, tt := range tests {
		name := tt.name
		if name == "" {
			name = fmt.Sprintf("test_index_%d", i)
		}
		t.Run(name, func(t *testing.T) {
			br := bufio.NewReader(tt.r)
			r := httpHostMatch{equals(tt.host), noopTarget{}}
			m, name := r.match(br)
			got := m != nil
			if got != tt.want {
				t.Fatalf("match = %v; want %v", got, tt.want)
			}
			if tt.want && name != tt.host {
				t.Fatalf("host = %s; want %s", name, tt.host)
			}
			get := make([]byte, 3)
			if _, err := io.ReadFull(br, get); err != nil {
				t.Fatal(err)
			}
			if string(get) != "GET" {
				t.Fatalf("did bufio.Reader consume bytes? got %q; want GET", get)
			}
		})
	}
}

type neverEnding byte

func (b neverEnding) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

func newLocalListener(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp", "[::1]:0")
		if err != nil {
			t.Fatal(err)
		}
	}
	return ln
}

const testFrontAddr = "127.0.0.1:7777"

func testListenFunc(t *testing.T, ln net.Listener) func(network, laddr string) (net.Listener, error) {
	return func(network, laddr string) (net.Listener, error) {
		if network != "tcp" {
			t.Errorf("got Listen call with network %q, not tcp", network)
			return nil, errors.New("invalid network")
		}
		if laddr != testFrontAddr {
			t.Fatalf("got Listen call with laddr %q, want %q", laddr, testFrontAddr)
		}
		return ln, nil
	}
}

func testProxy(t *testing.T, front net.Listener) *Proxy {
	return &Proxy{
		ListenFunc: testListenFunc(t, front),
	}
}

func TestProxyHTTP(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()

	backFoo := newLocalListener(t)
	defer backFoo.Close()
	backBar := newLocalListener(t)
	defer backBar.Close()

	p := testProxy(t, front)
	p.AddHTTPHostRoute(testFrontAddr, "127.0.0.1:9595", To(backFoo.Addr().String()))
	p.AddHTTPHostRoute(testFrontAddr, "bar.com", To(backBar.Addr().String()))
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer toFront.Close()

	const msg = "GET / HTTP/1.1\r\nHost: 127.0.0.1:9595\r\n\r\n"
	io.WriteString(toFront, msg)

	fromProxy, err := backFoo.Accept()
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(fromProxy, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q; want %q", buf, msg)
	}
}

func TestProxyPROXYOut(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()
	back := newLocalListener(t)
	defer back.Close()

	proxy := testProxy(t, front)
	proxy.AddRoute(testFrontAddr, &DialProxy{
		Addr:                 back.Addr().String(),
		ProxyProtocolVersion: 1,
	})
	if err := proxy.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	io.WriteString(toFront, "GET / HTTP/1.1\r\nHost: 127.0.0.1:9595\r\n\r\n")
	defer toFront.Close()

	fromProxy, err := back.Accept()
	if err != nil {
		t.Fatal(err)
	}

	bs, err := ioutil.ReadAll(fromProxy)
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf("PROXY TCP4 %s %d %s %d\r\nGET / HTTP/1.1\r\nHost: 127.0.0.1:9595\r\n\r\n", toFront.LocalAddr().(*net.TCPAddr).IP, toFront.LocalAddr().(*net.TCPAddr).Port, toFront.RemoteAddr().(*net.TCPAddr).IP, toFront.RemoteAddr().(*net.TCPAddr).Port)
	if string(bs) != want {
		t.Fatalf("got %q; want %q", bs, want)
	}
}

func TestProxyAlwaysMatch(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()
	back := newLocalListener(t)
	defer back.Close()

	p := testProxy(t, front)
	p.AddRoute(testFrontAddr, To(back.Addr().String()))
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer toFront.Close()

	fromProxy, err := back.Accept()
	if err != nil {
		t.Fatal(err)
	}
	const msg = "GET / HTTP/1.1\r\nHost: 127.0.0.1:9595\r\n\r\n"
	io.WriteString(toFront, msg)

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(fromProxy, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q; want %q", buf, msg)
	}
}
