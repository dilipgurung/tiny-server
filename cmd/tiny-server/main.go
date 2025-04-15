package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dilipgurung/tiny-server/internal/server"
)

var (
	port        = flag.String("p", "8000", "port to listen on")
	dir         = flag.String("d", "", "directory to serve files from")
	showVersion = flag.Bool("v", false, "show version")
)

func main() {
	flag.Usage = func() {
		printUsage()
		os.Exit(0)
	}
	flag.Parse()

	if *showVersion {
		versionInfo := server.NewVersionInfo(tinyServerVersion, goVersion)
		versionInfo.PrintSplash()
		return
	}

	serveDir := getServeDir(*dir)
	srv, err := server.NewServer(*port, serveDir)
	if err != nil {
		log.Fatal(err)
	}

	srv.PrintInfo(*port, serveDir)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	fmt.Println()
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Forced shutdown:", err)
	}
	log.Println("Server stopped")
}

func getServeDir(dir string) string {
	if dir == "" {
		if _, err := os.Stat("./public"); err == nil {
			return "./public"
		}
		return "."
	}
	return dir
}

func printUsage() {
	fmt.Printf("Usage: %s [options]\n", os.Args[0])
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("\nExample:")
	fmt.Printf("  %s -p 8000 -d ./public\n", os.Args[0])
}
