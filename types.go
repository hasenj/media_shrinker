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
}

// I would have liked to name this 'Size' but ..
type RectSize struct {
	Width, Height int
}

type Point struct {
	X, Y int
}

type Rect struct {
	Point // origin
	RectSize
}

type TuiScrollArea struct {
	Rect
	ScrollPosition int
	ScrollHeight int
}

type Tui struct {
	Screen tcell.Screen
	Processor *ProcessorData

	Messages []string

	Rect

	filesView, messagesView TuiScrollArea
}


type UpdateEvent struct {
	tcell.EventTime
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
