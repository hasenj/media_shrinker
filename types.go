package media_shrinker

type Processor struct {
	SrcDir, DstDir, TmpDir string
	DoClean                bool
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

	Deleted bool
}

type ProcessingRequest struct {
	InputPath  string
	OutputPath string
}

type ShrunkStats struct {
	Count int
	SizeBefore int
	SizeAfter int

	DeletedCount int
	DeletedSize int
}
