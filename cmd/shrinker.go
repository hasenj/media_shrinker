package main

import (
	"flag"
	"os"

	shrinker "go.hasen.dev/media_shrinker"
)

func main() {
	var opts shrinker.Options
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.StringVar(&opts.SrcDir, "src", ".", "The directory with the source media files")
	f.StringVar(&opts.DstDir, "dst", "./smaller", "The directory where compressed media files are to be placed")
	f.StringVar(&opts.TmpDir, "tmp", "./_temp_", "The directory where compressed media files are to be placed while being processed")
	f.BoolVar(&opts.DoClean, "clean", false, "Delete processed source media files")
	f.BoolVar(&opts.ReportOnly, "report-only", false, "Report current status without further processing any file")
	f.Parse(args)

	processor := shrinker.InitProcessorData(opts)

	var tui shrinker.Tui
	tui.Processor = processor
	tui.Init()

	go shrinker.StartProcessing(processor, &tui)
	tui.Loop()
}
