// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type rProxyTransport struct{}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getServers() {

	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.39", nil, nil)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	networkFilters := filters.NewArgs()
	networkFilters.Add("name", "dockprom_monitor-net")

	net, err := cli.NetworkList(ctx, types.NetworkListOptions{Filters: networkFilters})
	if err != nil {
		panic(err)
	}

	log.Println(net[0].Containers)

	for _, container := range net[0].Containers {
		fmt.Println(container.IPv4Address)
	}
}

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
	url, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(url)
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host
	proxy.Transport = &rProxyTransport{}
	proxy.ServeHTTP(res, req)
}

func (t *rProxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var start, connect, dns, tlsHandshake time.Time

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			log.Printf("DNS Done: %v\n", time.Since(dns))
		},

		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			log.Printf("TLS Handshake: %v\n", time.Since(tlsHandshake))
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			log.Printf("Connect time: %v\n", time.Since(connect))
		},

		GotFirstResponseByte: func() {
			log.Printf("Time from start to first byte: %v\n", time.Since(start))
		},
	}

	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))

	start = time.Now()

	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	log.Printf("Total time: %v\n", time.Since(start))

	return response, err
}

func parseRequestBody(request *http.Request) string {
	return request.URL.Query().Get("proxy")
}

// func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
// 	requestParameter := parseRequestBody(req)
// 	rproxy.LogRequestPayload(requestParameter, url)
// 	serveReverseProxy(url, res, req)
// }

func main() {
	// http.HandleFunc("/", handleRequestAndRedirect)
	// if err := http.ListenAndServe(":9090", nil); err != nil {
	// 	panic(err)
	// }
}
