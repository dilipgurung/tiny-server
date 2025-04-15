package server

import (
	"fmt"
	"runtime"
)

type VersionInfo struct {
	tinyServerVersion string
	goVersion         string
}

func NewVersionInfo(tinyServerVersion, goVersion string) VersionInfo {
	if len(tinyServerVersion) != 0 && len(goVersion) != 0 {
		return VersionInfo{
			tinyServerVersion: tinyServerVersion,
			goVersion:         goVersion,
		}
	}
	return VersionInfo{
		tinyServerVersion: "(unknown)",
		goVersion:         runtime.Version(),
	}
}

func (v *VersionInfo) PrintSplash() {
	fmt.Printf(`
  _____ _               ____                           
 |_   _(_)_ __  _   _  / ___|  ___ _ ____   _____ _ __ 
   | | | | '_ \| | | | \___ \ / _ \ '__\ \ / / _ \ '__|
   | | | | | | | |_| |  ___) |  __/ |   \ V /  __/ |   
   |_| |_|_| |_|\__, | |____/ \___|_|    \_/ \___|_|   
                |___/                                  
	`)
	fmt.Printf("\n   %s built with Go %s\n", v.tinyServerVersion, v.goVersion)
	fmt.Println("   A Simple and lightweight static HTTP server")
}
