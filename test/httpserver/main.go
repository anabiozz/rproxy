package main

import (
	"fmt"
	"net/http"
	"time"
)

type timeoutHandler struct{}

func (h timeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// defer r.Body.Close()

	fmt.Println("SADAWDFWFAW")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("DSADWF"))
}

func main() {

	mux := http.NewServeMux()
	h := timeoutHandler{}
	mux.Handle("/", h)

	server := &http.Server{
		// defines how long you allow a connection to be open during a client sends data
		ReadTimeout: 30 * time.Second,
		// it is in the other direction
		WriteTimeout: 10 * time.Second,
		// represents the time until the full request header (send by a client) should be read
		ReadHeaderTimeout: 20 * time.Second,
		Handler:           h,
		Addr:              "0.0.0.0:9595",
	}

	fmt.Println("server was up")
	fmt.Println(server.ListenAndServe())
}
