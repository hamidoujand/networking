package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n\t%s <group names>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

}

func main() {
	flag.Parse()

	groups := parseGroupNames(flag.Args())
	socket := filepath.Join(os.TempDir(), "creds.sock")

	addr, err := net.ResolveUnixAddr("unix", socket)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	s, err := net.ListenUnix("unix", addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)

	go func() {
		<-c
		_ = s.Close()
	}()

	fmt.Printf("listening on %s...\n", socket)

	for {
		conn, err := s.AcceptUnix()
		if err != nil {
			break
		}

		if allowed(conn, groups) {
			_, err := conn.Write([]byte("Welcome\n"))
			if err != nil {
				fmt.Println(err)
				_ = conn.Close()
				return
			}

			//handle the conn in goroutine
			go handler(conn)

		} else {
			_, err := conn.Write([]byte("Access Denied\n"))
			if err != nil {
				fmt.Println(err)
			}
			//close the conn in both case
			_ = conn.Close()
		}
	}
}

func handler(conn *net.UnixConn) {
	defer conn.Close()
	conn.Write([]byte("Passed Authentication"))
}

func parseGroupNames(args []string) map[string]struct{} {
	groups := make(map[string]struct{}, len(args))

	for _, arg := range args {
		group, err := user.LookupGroup(arg)
		if err != nil {
			fmt.Println(err)
			continue
		}

		groups[group.Gid] = struct{}{}
	}

	return groups
}

func allowed(conn *net.UnixConn, groups map[string]struct{}) bool {
	if conn == nil || groups == nil || len(groups) == 0 {
		return false
	}

	//access the file for the other peer.
	file, err := conn.File()
	if err != nil {
		fmt.Println(err)
		return false
	}

	defer func() { _ = file.Close() }()

	var uCred *unix.Ucred

	for {

		uCred, err = unix.GetsockoptUcred(int(file.Fd()), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			fmt.Println(err)
			return false
		}
		break
	}

	//pass the uid to get a *user.User back and on that we can get its groups
	u, err := user.LookupId(string(uCred.Uid))
	if err != nil {
		fmt.Println(err)
		return false
	}

	//groups
	gids, err := u.GroupIds()
	if err != nil {
		fmt.Println(err)
		return false
	}

	//if the user is in valid groups it can proceed
	for _, gid := range gids {
		if _, ok := groups[gid]; ok {
			return true
		}
	}

	return false
}
