package main

import (
	"flag"
	"fmt"
	"os"

	"openwatcher/desktop-app/internal/selfupdate"
)

func main() {
	var options selfupdate.HelperOptions
	flag.StringVar(&options.SourceDir, "source", "", "staged update source directory")
	flag.StringVar(&options.TargetPath, "target", "", "current app path")
	flag.StringVar(&options.LaunchPath, "launch", "", "updated app launch path")
	flag.StringVar(&options.StatusPath, "status", "", "status json path")
	flag.StringVar(&options.BackupRoot, "backup-root", "", "backup root")
	flag.StringVar(&options.Platform, "platform", "", "target platform")
	flag.StringVar(&options.Version, "version", "", "target version")
	flag.StringVar(&options.Artifact, "artifact", "", "target artifact")
	flag.Parse()

	if err := selfupdate.RunHelper(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
