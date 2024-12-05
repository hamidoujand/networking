package main

import (
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
