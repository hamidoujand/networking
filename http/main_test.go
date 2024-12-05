package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSimpleHTTPServer(t *testing.T) {
	server := http.Server{
		Addr: "127.0.0.1:8000",
		//takes a mux or any object capable of handling requests
		Handler:           http.TimeoutHandler(http.DefaultServeMux, time.Minute*2, ""),
		IdleTimeout:       time.Minute * 5,
		ReadHeaderTimeout: time.Minute,
	}

	//create a listener
	l, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	}()

}

func TestRestrictPrefix(t *testing.T) {

	//Req =========> stripPrefix ==========> restrictPrefix =========> fileServer

	handler := http.StripPrefix("/static/", restrictPrefix(".", http.FileServer(http.Dir("files"))))

	testCases := []struct {
		path string
		code int
	}{
		{"http://test/static/sage.svg", http.StatusOK},
		{"http://test/static/.secret", http.StatusNotFound},
		{"http://test/static/.dir/secret", http.StatusNotFound},
	}
	for i, c := range testCases {
		r := httptest.NewRequest(http.MethodGet, c.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		actual := w.Result().StatusCode
		if c.code != actual {
			t.Errorf("%d: expected %d; actual %d", i, c.code, actual)
		}
	}
}
