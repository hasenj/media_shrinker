package media_shrinker

type ProcessorInitOptions struct {
	SrcDir, DstDir, TmpDir string
	DoClean, ReportOnly    bool
}

type ProcessorData struct {
	InitOptions *ProcessorOptions
	InputFiles []MediaFile
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

	// FIXME use one ProcessingDone stage and use other fields to indicate other flags
	ProcessingError
	ProcessingSuccess
	AlreadyProcessed
)

type MediaFile struct {
	Type      MediaType
	Dir, Name string
	Size      int // in bytes

	Stage      ProcessingStage
	ShrunkSize int
	Error      error // if processing failed, or if processing worked but some other error occurred

	// When in progress, how far along are we!
	Percentage float32

	Deleted bool
}

type ProcessingRequest struct {
	Target *MediaFile

	InputPath  string
	OutputPath string
}

type ShrunkStats struct {
	Count int
	SizeBefore int
	SizeAfter int

	DeletedCount int
	DeletedSize int
	DeletedShrunkSize int
}
