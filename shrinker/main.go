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
	f.StringVar(&app.SrcDir, "src", ".", "The directory with the source media files")
	f.StringVar(&app.DstDir, "dst", "./smaller", "The directory where compressed media files are to be placed")
	f.StringVar(&app.TmpDir, "tmp", "./_temp_", "The directory where compressed media files are to be placed while being processed")
	f.BoolVar(&app.DoClean, "clean", false, "Delete processed source media files")
	f.BoolVar(&app.ReportOnly, "report-only", false, "Report current status without further processing any file")
	f.Parse(args)

	shrinker.DoProcess(&app)
}
