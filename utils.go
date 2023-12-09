package main

import (
	"hash/maphash"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"
)

func getServiceBaseURL(r *http.Request) string {
	if s.ServiceURL != "" {
		return s.ServiceURL
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if host == "localhost" {
			proto = "http"
		} else if strings.Index(host, ":") != -1 {
			// has a port number
			proto = "http"
		} else if _, err := strconv.Atoi(strings.ReplaceAll(host, ".", "")); err == nil {
			// it's a naked IP
			proto = "http"
		} else {
			proto = "https"
		}
	}
	return proto + "://" + host
}

func getIconURL(r *http.Request) string {
	for _, possibleIcon := range []string{"icon.png", "icon.jpg", "icon.jpeg", "icon.gif"} {
		if _, err := os.Stat(filepath.Join(s.CustomDirectory, possibleIcon)); err == nil {
			return getServiceBaseURL(r) + "/" + possibleIcon
		}
	}
	return ""
}

func pointerHasher[V any](_ maphash.Seed, k *V) uint64 {
	return uint64(uintptr(unsafe.Pointer(k)))
}
