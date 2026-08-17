package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	listenAddress = ":8080"
	contentRoot   = "/srv"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := checkHealth(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) != 1 {
		log.Fatalf("usage: %s [healthcheck]", os.Args[0])
	}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           newHandler(contentRoot),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("serving web content on %s", listenAddress)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newHandler(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/healthz" {
			writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ok\n"))
			return
		}

		cleanPath := filepath.Clean("/" + request.URL.Path)
		candidate := filepath.Join(root, strings.TrimPrefix(cleanPath, "/"))
		info, err := os.Stat(candidate)
		switch {
		case err == nil && !info.IsDir():
			if strings.HasPrefix(cleanPath, "/assets/") {
				writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				writer.Header().Set("Cache-Control", "no-cache")
			}
			files.ServeHTTP(writer, request)
		case err == nil || errors.Is(err, os.ErrNotExist):
			writer.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(writer, request, filepath.Join(root, "index.html"))
		default:
			http.Error(writer, "unable to read web content", http.StatusInternalServerError)
		}
	})
}

func checkHealth() error {
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		return fmt.Errorf("web health check: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("web health check returned %s", response.Status)
	}
	return nil
}
