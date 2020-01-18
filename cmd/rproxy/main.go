// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
	"github.com/anabiozz/rproxy/pkg/config/static"
	"github.com/anabiozz/rproxy/pkg/log"
	providerpkg "github.com/anabiozz/rproxy/pkg/provider"
	_ "github.com/anabiozz/rproxy/pkg/provider/all"
	"github.com/anabiozz/rproxy/pkg/provider/docker"
	"github.com/anabiozz/rproxy/pkg/provider/file"
	httprouter "github.com/anabiozz/rproxy/pkg/router/net/http"
	"github.com/spf13/viper"
)

type middleware func(http.Handler) http.Handler
type middlewares []middleware

type controller struct {
	logger log.Logger
}

func (mws middlewares) apply(handler http.Handler) http.Handler {
	if len(mws) == 0 {
		return handler
	}
	return mws[1:].apply(mws[0](handler))
}

func (c *controller) logging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func(start time.Time) {
			requestID := w.Header().Get("X-Request-Time")
			if requestID == "" {
				requestID = "unknown"
			}
			c.logger.Infof("PROXY: %s | TARGET: %s | SOURCE: %s | PATH: %s | METHOD: %s | USER AGENT: %s | TIME ELAPSED: %v",
				req.URL.Query().Get("proxy"), req.Header.Get("X-Origin-Host"), req.RemoteAddr, req.URL.Path, req.Method, req.UserAgent(), time.Since(start))
		}(time.Now())
		handler.ServeHTTP(w, req)
	})
}

func newProxyListener(url string) net.Listener {
	ln, err := net.Listen("tcp", url)
	if err != nil {
		fmt.Println(err)
		ln, err = net.Listen("tcp", "[::1]:0")
		if err != nil {
			fmt.Println(err)
		}
	}
	return ln
}

func listenFunc(ln net.Listener) func(network, laddr string) (net.Listener, error) {
	return func(network, laddr string) (net.Listener, error) {
		if network != "tcp" {
			fmt.Printf("got Listen call with network %q, not tcp\n", network)
			return nil, errors.New("invalid network")
		}
		return ln, nil
	}
}

func createProviders(
	ctx context.Context,
	providers *static.Providers,
	providerConfigurationCh chan *dynamic.Configuration,
	logger log.Logger) {

	for providerName, creator := range providerpkg.Providers {

		_, ok := providerpkg.Providers[providerName]
		if !ok {
			logger.Errorf("Undefined provider: %s", providerName)
		}
		providerCreator := creator()

		switch providerCreator.(type) {
		case *docker.Provider:
			providerCreator = providers.Docker
		case *file.Provider:
			providerCreator = providers.File
		}

		err := providerCreator.Provide(ctx, providerConfigurationCh)
		if err != nil {
			logger.Error(err)
		}

	}
}

func main() {

	ctx := context.Background()
	logger := log.WithContext(ctx)

	// ################################################################
	// # Config
	// ################################################################

	viper.SetConfigName("sample")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.Error(err)
		os.Exit(-1)
	}

	var cfg static.Configuration
	err := viper.Unmarshal(&cfg)
	if err != nil {
		logger.Error(err)
		os.Exit(-1)
	}

	// ################################################################
	// # Provider
	// ################################################################

	config := dynamic.Configuration{}

	providerConfigurationCh := make(chan *dynamic.Configuration, 100)
	errorCh := make(chan error)
	config.Services = make(map[string]*dynamic.Service)

	go createProviders(ctx, cfg.Providers, providerConfigurationCh, logger)

	go func() {
		select {
		case providercfg := <-providerConfigurationCh:

			for serviceName, service := range providercfg.Services {

				for _, server := range service.Servers {

					go func(server dynamic.Server, serviceName string) {

						endpoint := (*cfg.EntryPoints)[serviceName]

						ctxLog := log.NewContext(ctx, log.Str("function", "generateProxy"))
						logger := log.WithContext(ctxLog)

						front := newProxyListener(endpoint.Address)
						defer front.Close()

						proxy := &httprouter.Proxy{
							ListenFunc: listenFunc(front),
						}

						// proxy.AddHTTPHostRoute(endpoint.Address, httpserver1, httprouter.To(server.URL))
						// proxy.AddHTTPHostRoute(endpoint.Address, httpserver2, httprouter.To(server.URL))
						proxy.AddRoute(endpoint.Address, httprouter.To(server.URL))

						if err := proxy.Start(); err != nil {
							logger.Error(err)
						}

						toProxy, err := net.Dial("tcp", front.Addr().String())
						if err != nil {
							logger.Error("Dial", err)
						}

						for {
							// accept connection on port
							conn, err := front.Accept()
							if err != nil {
								logger.Error("Accept", err)
							}

							// will listen for message to process ending in newline (\n)
							message, err := bufio.NewReader(conn).ReadString('\n')
							if err != nil {
								logger.Error("ReadString", err)
							}

							toProxy.Write([]byte(message))

							toProxy.Close()
						}

					}(server, serviceName)
				}
			}

		case err := <-errorCh:
			logger.Error(err)
		}
	}()

	logger.Info("SERVICE STARTED")
	defer logger.Info("SERVICE ENDED")

	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		// c := &controller{logger: logger}
		logger.Info("TRANSPORT: 'HTTP', ADDR: '127.0.0.1:9090'")
		server := &http.Server{
			// Handler:        (middlewares{c.logging}).apply(router),
			Addr:           "127.0.0.1:9090",
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    20 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		errs <- server.ListenAndServe()
	}()

	logger.Info(<-errs)
}

// formatRequest generates ascii representation of a request
func formatRequest(req *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", req.Method, req.URL, req.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", req.Host))
	// Loop through headers
	for name, headers := range req.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if req.Method == "POST" {
		req.ParseForm()
		request = append(request, "\n")
		request = append(request, req.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}
