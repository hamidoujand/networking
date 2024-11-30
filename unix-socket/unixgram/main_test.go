package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestEchoServerUnixDatagram(t *testing.T) {
	dir, err := os.MkdirTemp("", "echo_unixgram")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverSocket := filepath.Join(dir, fmt.Sprintf("server%d.sock", os.Getpid()))
	serverAddr, err := datagramEchoServer(ctx, "unixgram", serverSocket)
	if err != nil {
		t.Fatal(err)
	}

	//now the file is created we can change its mod
	if err := os.Chmod(serverSocket, os.ModeSocket|0622); err != nil {
		t.Fatal(err)
	}

	clientSocket := filepath.Join(dir, fmt.Sprintf("client%d.sock", os.Getpid()))
	client, err := net.ListenPacket("unixgram", clientSocket)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = client.Close() }()

	if err := os.Chmod(clientSocket, os.ModeSocket|0622); err != nil {
		t.Fatal(err)
	}

	msg := []byte("ping")
	for range 3 {
		_, err := client.WriteTo(msg, serverAddr)
		if err != nil {
			t.Fatal(err)
		}
	}

	buf := make([]byte, 1024)
	for range 3 {
		n, addr, err := client.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		if addr.String() != serverAddr.String() {
			t.Fatalf("received reply from %q instead of %q",
				addr, serverAddr)
		}
		if !bytes.Equal(msg, buf[:n]) {
			t.Fatalf("expected reply %q; actual reply %q", msg,
				buf[:n])
		}
	}
}
