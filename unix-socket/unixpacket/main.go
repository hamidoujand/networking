package main

import (
	"context"
	"fmt"
	"io"
	"net"
)

func streamingEchoServer(ctx context.Context, network string, addr string) (net.Addr, error) {
	s, err := net.Listen(network, addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	//server
	go func() {
		go func() {
			//wait til ctx is done then close the server
			<-ctx.Done()
			_ = s.Close()
		}()

		//server
		for {
			conn, err := s.Accept()
			if err != nil {
				return
			}

			//handler
			go func() {
				defer func() { _ = conn.Close() }()

				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil && err != io.EOF {
						return
					}
					//write to the same conn
					if _, err := conn.Write(buf[:n]); err != nil {
						return
					}
				}
			}()
		}
	}()

	return s.Addr(), nil
}
