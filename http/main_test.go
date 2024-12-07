package main

import (
	"fmt"
	"io"
	"io/ioutil"
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

func TestSimpleMuxWithMiddleware(t *testing.T) {
	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	serveMux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Hello friend.")
	})

	serveMux.HandleFunc("/hello/there/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Why, hello there.")
	})

	//apply middlewares
	mux := applyMiddlewares(serveMux, drainAndClose)

	testCases := []struct {
		path     string
		response string
		code     int
	}{
		{"http://test/", "", http.StatusNoContent},
		{"http://test/hello", "Hello friend.", http.StatusOK},
		{"http://test/hello/there/", "Why, hello there.", http.StatusOK},
		{"http://test/hello/there", "<a href=\"/hello/there/\">Moved Permanently</a>.\n\n",
			http.StatusMovedPermanently},
		{"http://test/hello/there/you", "Why, hello there.", http.StatusOK},
		{"http://test/hello/and/goodbye", "", http.StatusNoContent},
		{"http://test/something/else/entirely", "", http.StatusNoContent},
		{"http://test/hello/you", "", http.StatusNoContent},
	}

	for i, c := range testCases {
		r := httptest.NewRequest(http.MethodGet, c.path, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		resp := w.Result()

		if actual := resp.StatusCode; c.code != actual {
			t.Errorf("%d: expected code %d; actual %d", i, c.code, actual)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if actual := string(b); c.response != actual {
			t.Errorf("%d: expected response %q; actual %q", i,
				c.response, actual)
		}
	}
}

func drainAndClose(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//call next handler
		next.ServeHTTP(w, r)

		//drain body
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	})
}

type Middleware func(http.Handler) http.Handler

func applyMiddlewares(mux http.Handler, mids ...Middleware) http.Handler {
	for i := len(mids) - 1; i >= 0; i-- {
		m := mids[i]
		if m != nil {
			mux = m(mux)
		}
	}
	return mux
}
