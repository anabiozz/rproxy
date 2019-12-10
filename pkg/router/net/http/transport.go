package http

import (
	"github.com/anabiozz/rproxy/pkg/log"
	"net/http"
)

type rProxyTransport struct {
	logger log.Logger
	*http.Transport
}

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
func (tr *rProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// var start, connect, dns, tlsHandshake time.Time

	// trace := &httptrace.ClientTrace{
	// 	DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
	// 	DNSDone: func(ddi httptrace.DNSDoneInfo) {
	// 		tr.logger.Infof("DNS DONE: %v\n", time.Since(dns))
	// 	},

	// 	TLSHandshakeStart: func() { tlsHandshake = time.Now() },
	// 	TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
	// 		tr.logger.Infof("TLS HANDSHAKE: %v\n", time.Since(tlsHandshake))
	// 	},

	// 	ConnectStart: func(network, addr string) { connect = time.Now() },
	// 	ConnectDone: func(network, addr string, err error) {
	// 		tr.logger.Infof("CONNECT TIME: %v\n", time.Since(connect))
	// 	},

	// 	GotFirstResponseByte: func() {
	// 		tr.logger.Infof("TIME FROM START TO FIRST BYTE: %v\n", time.Since(start))
	// 	},
	// }

	// req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// start = time.Now()

	response, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		tr.logger.Error(err)
		return nil, err
	}

	// tr.logger.Infof("TOTAL TIME: %v\n", time.Since(start))

	return response, err
}
