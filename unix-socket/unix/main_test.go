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

func TestEchoServerUnit(t *testing.T) {
	dir, err := os.MkdirTemp("", "echo_unix")
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

	socket := filepath.Join(dir, fmt.Sprintf("%d.socket", os.Getpid()))

	addr, err := streamingEchoServer(ctx, "unix", socket)
	if err != nil {
		t.Fatal(err)
	}

	//after this the file has been created so now we can chmod it
	if err := os.Chmod(socket, os.ModeSocket|0666); err != nil {
		t.Fatal(err)
	}

	//now we need to dial it
	conn, err := net.Dial("unix", addr.String())
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
	//read 1 time
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	expected := bytes.Repeat(msg, 3)
	if !bytes.Equal(buf[:n], expected) {
		t.Fatal("not equal")
	}
}
