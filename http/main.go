package main

import (
	"fmt"
	"net/http"
	"path"
	"strings"
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
