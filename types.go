package media_shrinker

import "time"

import "github.com/gdamore/tcell/v2"

type Options struct {
	SrcDir, DstDir, TmpDir string
	DoClean, ReportOnly    bool
}

type ProcessorData struct {
	Options
	MediaFiles []MediaFile
	CurrentIndex int // index of currently processing file from the list
}

type Tui struct {
	Screen tcell.Screen
	Processor *ProcessorData

	Messages []string

	Width, Height int

	// scrolling
	ScrollPosition int
	ScrollHeight int
}

type MediaType int

const (
	UnknownType MediaType = iota
	Video
	JPG
	PNG
)

type ProcessingStage int

const (
	Waiting ProcessingStage = iota
	ProcessingInProgress

	ProcessingError	  // attempted to process but failed
	ProcessingSuccess // processed this time and succeeded
	AlreadyProcessed  // processed from previous runs
)

type UI interface {
	LogError(format string, a ...interface{})
	Update()
}

type MediaFile struct {
	Type      MediaType
	Dir, Name string
	Size      int // in bytes

	Stage      ProcessingStage
	ShrunkSize int
	Error      error // if processing failed, or if processing worked but some other error occurred

	// When in progress, how far along are we!
	Percentage float64

	// For videos, duration processed (in seconds)
	Processed float64

	Deleted bool

	StartTime, EndTime time.Time
}

type ProcessingRequest struct {
	Target *MediaFile

	InputPath  string
	OutputPath string

	UI UI
}

type ShrunkStats struct {
	Count int
	SizeBefore int
	SizeAfter int

	DeletedCount int
	DeletedSize int
	DeletedShrunkSize int
}
