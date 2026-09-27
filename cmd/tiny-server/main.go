package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dilipgurung/tiny-server/internal/server"
)

// Process exit statuses. Startup failures must not look like success to
// scripts, package managers and service supervisors.
const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args and serves until interrupted, returning the process exit
// status. Keeping this out of main lets tests exercise the exit statuses
// without spawning a subprocess.
func run(args []string, stdout, stderr io.Writer) int {
	// Usage and parse errors are rendered here rather than by the flag
	// package so help goes to stdout and errors go to stderr.
	fs := flag.NewFlagSet("tiny-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	port := fs.String("p", "8000", "port to listen on")
	dir := fs.String("d", "", "directory to serve files from")
	showVersion := fs.Bool("v", false, "show version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout, fs)
			return exitOK
		}
		_, _ = fmt.Fprintf(stderr, "%v\n\n", err)
		printUsage(stderr, fs)
		return exitUsage
	}

	if *showVersion {
		NewVersionInfo(tinyServerVersion, goVersion).PrintSplashTo(stdout)
		return exitOK
	}

	serveDir := getServeDir(*dir)
	srv, err := server.NewServer(*port, serveDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return exitFail
	}

	// Bind before printing the banner so an unusable port fails loudly
	// instead of scrolling past a QR code for an address nothing is on.
	if err := srv.Listen(); err != nil {
		_, _ = fmt.Fprintf(stderr, "cannot listen on port %q: %v\n", *port, err)
		return exitFail
	}

	srv.PrintInfo(srv.Port(), serveDir)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	status := exitOK
	select {
	case <-done:
	case err := <-errCh:
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			status = exitFail
		}
	}
	_, _ = fmt.Fprintln(stdout)
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "forced shutdown: %v\n", err)
		return exitFail
	}
	log.Println("Server stopped")
	return status
}

// getServeDir picks the directory to serve: an explicit -d, else ./public
// when it exists, else the working directory. The IsDir check matters
// because a regular file named "public" would otherwise be chosen and then
// rejected at startup, leaving the command unusable in that directory.
func getServeDir(dir string) string {
	if dir == "" {
		if info, err := os.Stat("./public"); err == nil && info.IsDir() {
			return "./public"
		}
		return "."
	}
	return dir
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	_, _ = fmt.Fprintf(w, "Usage: %s [options]\n", os.Args[0])
	_, _ = fmt.Fprintln(w, "Options:")
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
	_, _ = fmt.Fprintln(w, "\nExample:")
	_, _ = fmt.Fprintf(w, "  %s -p 8000 -d ./public\n", os.Args[0])
}
