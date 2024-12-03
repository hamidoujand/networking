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

func TestEchoServerUnixPacket(t *testing.T) {
	dir, err := os.MkdirTemp("", "echo_unixpacket")
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

	socket := filepath.Join(dir, fmt.Sprintf("%d.sock", os.Getpid()))

	rAddr, err := streamingEchoServer(ctx, "unixpacket", socket)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(socket, os.ModeSocket|0666); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("unixpacket", rAddr.String())
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = conn.Close() }()

	msg := []byte("ping")

	for range 3 {
		_, err := conn.Write(msg)
		if err != nil {
			t.Fatal(err)
		}
	}

	buf := make([]byte, 1024)
	for range 3 {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf[:n], msg) {
			t.Fatalf("expected msg to be %s, got %s\n", string(msg), string(buf[:n]))
		}
	}
}
