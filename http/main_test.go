package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
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
		b, err := io.ReadAll(resp.Body)
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

func TestClientTLS(t *testing.T) {
	ts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.TLS == nil {
				u := "https://" + r.Host + r.RequestURI
				http.Redirect(w, r, u, http.StatusMovedPermanently)
				return
			}

			w.WriteHeader(http.StatusOK)
		}),
	)

	defer ts.Close()

	//this client is configured to trust this "tls" server and it's self-signed certificate.
	resp, err := ts.Client().Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; actual status %d",
			http.StatusOK, resp.StatusCode)
	}

	//let's try a normal client
	tp := http.Transport{
		TLSClientConfig: &tls.Config{
			CurvePreferences: []tls.CurveID{tls.CurveP256},
			MinVersion:       tls.VersionTLS12,
		},
	}

	if err := http2.ConfigureTransport(&tp); err != nil {
		t.Fatal(err)
	}

	client2 := &http.Client{Transport: &tp}

	_, err = client2.Get(ts.URL)
	if err == nil || !strings.Contains(err.Error(),
		"certificate signed by unknown authority") {
		t.Fatalf("expected unknown authority error; actual: %q", err)
	}

	//now skip the verification
	tp.TLSClientConfig.InsecureSkipVerify = true

	resp, err = client2.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; actual status %d",
			http.StatusOK, resp.StatusCode)
	}
}

func TestClientTLSGoogle(t *testing.T) {

	dialer := &net.Dialer{
		Timeout: time.Second * 30,
	}

	tlsCong := tls.Config{
		CurvePreferences: []tls.CurveID{tls.CurveP256},
		MinVersion:       tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", "www.google.com:443", &tlsCong)
	if err != nil {
		t.Fatal(err)
	}

	// TLS details about the connection
	state := conn.ConnectionState()

	t.Logf("TLS 1.%d", state.Version-tls.VersionTLS10)
	t.Log(tls.CipherSuiteName(state.CipherSuite))
	t.Log(state.VerifiedChains[0][0].Issuer.Organization[0])

	_ = conn.Close()
}

func TestEchoTLSServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverAddr := "localhost:34443"
	maxIdle := time.Second
	server := NewTLSServer(ctx, serverAddr, maxIdle, nil)

	done := make(chan struct{})

	go func() {
		err := server.ListenAndServeTLS("cert.pem", "key.pem")
		if err != nil {
			t.Error(err)
			return
		}

		done <- struct{}{}

	}()
	//waits for server to become ready
	server.Ready()

	//pinning certificate
	cert, err := os.ReadFile("cert.pem")
	if err != nil {
		t.Fatal(err)
	}

	//create a cert pool
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		t.Fatalf("failed to append certificate into pool: %s", err)
	}

	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.CurveP256},
		MinVersion:       tls.VersionTLS12,
		RootCAs:          certPool,
	}

	// pass tls.Dial the tls.Config with the pinned server certificate 1.
	// Your TLS client authenticates the server’s certificate without having
	// to resort to using InsecureSkipVerify and all the insecurity that option
	// introduces.
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		t.Fatal(err)
	}

	hello := []byte("hello")
	_, err = conn.Write(hello)
	if err != nil {
		t.Fatal(err)
	}

	bs := make([]byte, 1024)
	n, err := conn.Read(bs)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(bs[:n], hello) {
		t.Fatalf("expected %q; actual %q", hello, bs[:n])
	}

	time.Sleep(2 * maxIdle)

	_, err = conn.Read(bs)
	if err != io.EOF {
		t.Fatal(err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	cancel()
	<-done
}
