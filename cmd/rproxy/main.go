// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
	"github.com/anabiozz/rproxy/pkg/config/static"
	"github.com/anabiozz/rproxy/pkg/log"
	providerpkg "github.com/anabiozz/rproxy/pkg/provider"
	_ "github.com/anabiozz/rproxy/pkg/provider/all"
	"github.com/anabiozz/rproxy/pkg/provider/docker"
	"github.com/anabiozz/rproxy/pkg/provider/file"
	httprouter "github.com/anabiozz/rproxy/pkg/router/net"
	"github.com/spf13/viper"
)

func newProxyListener(url string, logger log.Logger) net.Listener {

	addr, err := net.ResolveTCPAddr("tcp", url)
	if err != nil {
		logger.Error(err)
	}

	ln, err := net.Listen("tcp4", addr.String())
	if err != nil {
		logger.Error(err)
		ln, err = net.Listen("tcp6", addr.String())
		if err != nil {
			logger.Error(err)
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
	ctxLog := log.NewContext(ctx, log.Str("function", "main"))
	logger := log.WithContext(ctxLog)

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

	go func(ctx context.Context, logger log.Logger) {
		select {
		case providercfg := <-providerConfigurationCh:

			for serviceName, service := range providercfg.Services {

				for _, server := range service.Servers {

					go func(server dynamic.Server, serviceName string, logger log.Logger) {

						endpoint := (*cfg.EntryPoints)[serviceName]

						front := newProxyListener(endpoint.Address, logger)
						defer front.Close()

						proxy := &httprouter.Proxy{
							ListenFunc: listenFunc(front),
						}

						proxy.AddRoute(endpoint.Address, &httprouter.DialProxy{
							Addr: server.URL,
						})

						if err := proxy.Start(ctx); err != nil {
							logger.Error(err)
						}

						for {
						}

					}(server, serviceName, logger)
				}
			}

		case err := <-errorCh:
			logger.Error(err)
		}
	}(ctx, logger)

	logger.Info("SERVICE STARTED")
	defer logger.Info("SERVICE ENDED")

	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		logger.Info("TRANSPORT: 'HTTP', ADDR: '127.0.0.1:9090'")
		server := &http.Server{
			Addr:           "127.0.0.1:9090",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    20 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		errs <- server.ListenAndServe()
	}()

	logger.Info(<-errs)
}
