// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/anabiozz/rproxy/pkg/config"
	configpkg "github.com/anabiozz/rproxy/pkg/config"
	"github.com/anabiozz/rproxy/pkg/log"
	providerpkg "github.com/anabiozz/rproxy/pkg/provider"
	_ "github.com/anabiozz/rproxy/pkg/provider/all"
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

func newLocalListener() net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp", "[::1]:0")
		if err != nil {
			fmt.Println(err)
		}
	}
	return ln
}

var testFrontAddr = ""

func listenFunc(ln net.Listener) func(network, laddr string) (net.Listener, error) {
	return func(network, laddr string) (net.Listener, error) {
		if network != "tcp" {
			fmt.Printf("got Listen call with network %q, not tcp\n", network)
			return nil, errors.New("invalid network")
		}
		// if laddr != testFrontAddr {
		// 	fmt.Printf("got Listen call with laddr %q, want %q\n", laddr, testFrontAddr)
		// }
		return ln, nil
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

	var cfg config.Configuration
	err := viper.Unmarshal(&cfg)
	if err != nil {
		logger.Error(err)
		os.Exit(-1)
	}

	// ################################################################
	// # Provider
	// ################################################################

	config := configpkg.ProviderConfiguration{}

	providerConfigurationCh := make(chan configpkg.ProviderConfiguration, 100)
	errorCh := make(chan error)
	config.Services = make(map[string]configpkg.Service)
	// router := http.NewServeMux()

	go createProviders(ctx, cfg.Providers, providerConfigurationCh, logger)

	go func() {
		select {
		case <-providerConfigurationCh:

			/**
			TODO реализовать концепцию эндпоитов, например выставлять
			порт 8080 и перенаправлять с него на нужный сервис/сы
			в зависимости сколько поднято экземпляров сервиса

			В контейнерах в лейблах описываем с какого url прокся перенаправляет трафик
			на этот контейнер

			"rpoxy.routers.container.rule=Host(`container.docker.localhost`)"

			Если http запрос то можно в загаловке host присылать container.docker.localhost
			*/

			// router.Handle("/", generateProxy(ctx, config.Services, ""))
			ctxLog := log.NewContext(ctx, log.Str("function", "generateProxy"))
			logger := log.WithContext(ctxLog)

			front := newLocalListener()
			defer front.Close()

			proxy := &httprouter.Proxy{
				ListenFunc: listenFunc(front),
			}

			urlDst := "127.0.0.1:9595"
			urlDst2 := "127.0.0.1:9594"
			urlDst3 := "127.0.0.1:9593"
			urlProxy := "127.0.0.1:7777"
			urlProxy2 := "127.0.0.1:7778"

			proxy.AddHTTPHostRoute(urlProxy, urlProxy, httprouter.To(urlDst))
			proxy.AddHTTPHostRoute(urlProxy2, urlProxy2, httprouter.To(urlDst2))
			proxy.AddRoute(urlProxy, httprouter.To(urlDst3))

			if err := proxy.Start(); err != nil {
				logger.Error(err)
			}

			toProxy, err := net.Dial("tcp", front.Addr().String())
			if err != nil {
				logger.Error("Dial", err)
			}

			reqs := formatRequest(req)
			reqs += "\n\n"
			logger.Info("reqs ", reqs)

			toProxy.Write([]byte(reqs))

			defer toProxy.Close()

			br := bufio.NewReader(toProxy)
			resp, err := http.ReadResponse(br, req)
			if err != nil {
				logger.Error("ReadResponse ", err)
			}

			var body []byte
			if resp != nil && resp.Body != nil {
				body, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					logger.Error("ReadAll ", err)
				}
			}

			res.Write(body)

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

func generateProxy(ctx context.Context, services map[string]config.Service, providerName string) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		ctxLog := log.NewContext(ctx, log.Str("function", "generateProxy"))
		logger := log.WithContext(ctxLog)

		front := newLocalListener()
		defer front.Close()

		proxy := &httprouter.Proxy{
			ListenFunc: listenFunc(front),
		}

		urlDst := "127.0.0.1:9595"
		urlDst2 := "127.0.0.1:9594"
		urlDst3 := "127.0.0.1:9593"
		urlProxy := "127.0.0.1:7777"
		urlProxy2 := "127.0.0.1:7778"

		proxy.AddHTTPHostRoute(urlProxy, urlProxy, httprouter.To(urlDst))
		proxy.AddHTTPHostRoute(urlProxy2, urlProxy2, httprouter.To(urlDst2))
		proxy.AddRoute(urlProxy, httprouter.To(urlDst3))

		if err := proxy.Start(); err != nil {
			logger.Error(err)
		}

		toProxy, err := net.Dial("tcp", front.Addr().String())
		if err != nil {
			logger.Error("Dial", err)
		}

		reqs := formatRequest(req)
		reqs += "\n\n"
		logger.Info("reqs ", reqs)

		toProxy.Write([]byte(reqs))

		defer toProxy.Close()

		br := bufio.NewReader(toProxy)
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			logger.Error("ReadResponse ", err)
		}

		var body []byte
		if resp != nil && resp.Body != nil {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Error("ReadAll ", err)
			}
		}

		res.Write(body)
	})
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}

func createProviders(
	ctx context.Context,
	providers map[string]map[string]interface{},
	providerConfigurationCh chan configpkg.ProviderConfiguration,
	logger log.Logger) {

	for providerName, provider := range providers {

		creator, ok := providerpkg.Providers[providerName]
		if !ok {
			logger.Errorf("Undefined provider: %s", providerName)
		}
		providerCreator := creator()

		t := reflect.ValueOf(providerCreator).Elem()
		for k, v := range provider {
			val := t.FieldByName(strings.Title(k))
			val.Set(reflect.ValueOf(v))
		}

		err := providerCreator.Provide(ctx, providerConfigurationCh)
		if err != nil {
			logger.Error(err)
		}

		// router.Handle("/", httprouter.GenerateProxy(ctx, config.Services, providerName))
	}
}
