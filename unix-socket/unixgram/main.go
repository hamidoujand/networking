//go:build darwin || linux

package main

import (
	"context"
	"net"
	"os"
)

func datagramEchoServer(ctx context.Context, network string, addr string) (net.Addr, error) {
	s, err := net.ListenPacket(network, addr)
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			//wait for ctx
			<-ctx.Done()
			//then close the server
			_ = s.Close()
			//since you don’t use net.Listen or net.ListenUnix to create the listener,
			//Go won’t clean up the socket file for you when your server is done with it.
			if network == "unixgram" {
				_ = os.Remove(addr)
			}
		}()

		buf := make([]byte, 1024)

		for {
			n, clientAddr, err := s.ReadFrom(buf)
			if err != nil {
				return
			}

			//echo back
			_, err = s.WriteTo(buf[:n], clientAddr)
			if err != nil {
				return
			}
		}
	}()

	return s.LocalAddr(), nil
}
