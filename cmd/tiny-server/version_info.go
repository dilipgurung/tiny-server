package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

type VersionInfo struct {
	tinyServerVersion string
	goVersion         string
}

func NewVersionInfo(tinyServerVersion, goVersion string) *VersionInfo {
	if tinyServerVersion != "" && goVersion != "" {
		return &VersionInfo{
			tinyServerVersion: tinyServerVersion,
			goVersion:         goVersion,
		}
	}
	return &VersionInfo{
		tinyServerVersion: "(unknown)",
		goVersion:         runtime.Version(),
	}
}

func (v *VersionInfo) PrintSplash() {
	v.PrintSplashTo(os.Stdout)
}

// PrintSplashTo writes the splash banner to w.
func (v *VersionInfo) PrintSplashTo(w io.Writer) {
	_, _ = fmt.Fprintf(w, `
  _____ _               ____                           
 |_   _(_)_ __  _   _  / ___|  ___ _ ____   _____ _ __ 
   | | | | '_ \| | | | \___ \ / _ \ '__\ \ / / _ \ '__|
   | | | | | | | |_| |  ___) |  __/ |   \ V /  __/ |   
   |_| |_|_| |_|\__, | |____/ \___|_|    \_/ \___|_|   
                |___/                                  
	`)
	_, _ = fmt.Fprintf(w, "\n   %s built with Go %s\n", v.tinyServerVersion, v.goVersion)
	_, _ = fmt.Fprintln(w, "   A Simple and lightweight static HTTP server")
}
