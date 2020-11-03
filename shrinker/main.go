package main

import (
	"flag"
	"os"

	shrinker "go.hasen.dev/media_shrinker"
)

func main() {
	var app shrinker.Processor
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.StringVar(&app.SrcDir, "src", ".", "The directory with the source video files")
	f.StringVar(&app.DstDir, "dst", "./smaller", "The directory where compressed videos are to be placed")
	f.StringVar(&app.TmpDir, "tmp", "./_temp_", "The directory where compressed videos are to be placed")
	f.BoolVar(&app.DoClean, "clean", false, "Whether to clean converted files or not")
	f.Parse(args)

	shrinker.DoProcess(&app)
}
