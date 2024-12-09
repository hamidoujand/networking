package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func restrictPrefix(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		separated := strings.Split(cleaned, "/")

		for _, p := range separated {
			if strings.HasPrefix(p, prefix) {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
		}

		//next
		next.ServeHTTP(w, r)
	})
}

func withPusher(w http.ResponseWriter, r *http.Request) {

	//if the response writer is an http pusher,it can push resources to the client

	if pusher, ok := w.(http.Pusher); ok {
		//push the resources into the client's push cache
		targets := []string{
			"/static/style.css",
			"/static/hiking.svg",
		}

		for _, target := range targets {
			//writing the content of those files into client's connection buffer
			if err := pusher.Push(target, nil); err != nil {
				fmt.Printf("push failed %s: %s\n", target, err)
			}
		}
	}

	http.ServeFile(w, r, "index.html")
}

type Server struct {
	ctx       context.Context
	ready     chan struct{}
	addr      string
	maxIdle   time.Duration
	tlsConfig *tls.Config
}

func NewTLSServer(ctx context.Context, address string, maxIdle time.Duration, tlsConf *tls.Config) *Server {
	return &Server{
		ctx:       ctx,
		ready:     make(chan struct{}),
		addr:      address,
		maxIdle:   maxIdle,
		tlsConfig: tlsConf,
	}
}

func (s *Server) Ready() {
	if s.ready != nil {
		<-s.ready
	}
}

func (s *Server) ListenAndServeTLS(cert, key string) error {
	if s.addr == "" {
		s.addr = "localhost:443"
	}

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("binding tcp %s: %w", s.addr, err)
	}

	if s.ctx != nil {
		go func() {
			<-s.ctx.Done()
			//close
			_ = l.Close()
		}()
	}

	return s.ServeTLS(l, cert, key)
}

func (s *Server) ServeTLS(l net.Listener, cert, key string) error {
	if s.tlsConfig == nil {
		s.tlsConfig = &tls.Config{
			CurvePreferences:         []tls.CurveID{tls.CurveP256},
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
		}
	}

	//check if the static certificate is not provided and also there is no functions
	//configured to dynamically fetch certificate, If neither is set
	//the code will attempt to load certificates from provided files
	if len(s.tlsConfig.Certificates) == 0 && s.tlsConfig.GetCertificate == nil {
		certificate, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return fmt.Errorf("loading key pair: %w", err)
		}

		// Add the loaded certificate to the TLS configuration
		s.tlsConfig.Certificates = []tls.Certificate{certificate}

	}

	listenerTLS := tls.NewListener(l, s.tlsConfig)
	if s.ready != nil {
		close(s.ready)
	}

	for {
		// Since we are using a TLS-aware listener, it returns connection objects with
		// underlying TLS support
		conn, err := listenerTLS.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		//handler
		go func() {
			defer func() { _ = conn.Close() }()

			for {
				if s.maxIdle > 0 {
					//set the deadline on conn
					if err := conn.SetDeadline(time.Now().Add(s.maxIdle)); err != nil {
						return
					}
				}

				buf := make([]byte, 1024)
				//this is a blocking call so we only wait till the deadline exceeds
				n, err := conn.Read(buf)
				if err != nil {
					return
				}

				_, err = conn.Write(buf[:n])
				if err != nil {
					return
				}
			}
		}()
	}
}

func generatingCertificate(hosts []string) error {

	//generates a random number between [0,max-1], here it is [0,1*2^128]
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating int: %w", err)
	}

	notBefore := time.Now()

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Foo Bar"},
		},

		NotBefore: notBefore,
		NotAfter:  notBefore.Add(10 * 365 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			//if its IP address
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			//anything else
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate private key: %w", err)
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &private.PublicKey, private)
	if err != nil {
		return fmt.Errorf("generating certificate: %w", err)
	}

	//create a file
	certFile, err := os.Create("cert.pem")
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return fmt.Errorf("encode into pem: %w", err)
	}

	fmt.Println("wrote cert.pem")

	keyFile, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}

	defer keyFile.Close()

	privateBS, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return fmt.Errorf("private key byte slice: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privateBS}); err != nil {
		return fmt.Errorf("encode private key into pem: %w", err)
	}

	fmt.Println("wrote key.pem")
	return nil
}

func main() {
	generatingCertificate([]string{"localhost"})
}
