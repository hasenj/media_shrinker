package media_shrinker

import (
	"fmt"
	"log"
	"time"
	"os"
	"path"
	"strings"
	"sort"
)

func (m MediaType) String() string {
	switch m {
		case UnknownType: return "!unknown!"
		case Video: return "video"
		case PNG: return "png"
		case JPG: return "jpg"
	}
	return "!Unhandled-Case!"
}

func guessMediaType(filename string) MediaType {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
		case ".mp4": return Video
		case ".jpg", ".jpeg": return JPG
		case ".png": return PNG
		default: return UnknownType
	}
}

func ListMediaFiles(dir string) ([]MediaFile, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory %s: %w", dir, err)
	}
	entries, err := f.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("Error listing directory %s: %w", dir, err)
	}
	files := make([]MediaFile, 0)
	for _, fentry := range entries {
		name := fentry.Name()
		mtype := guessMediaType(name)
		if mtype == UnknownType {
			continue
		}
		size := int(fentry.Size())
		vfile := MediaFile{
			Dir:  dir,
			Name: name,
			Type: mtype,
			Size: size,
		}
		files = append(files, vfile)
	}
	return files, nil
}

const KB = 1 << 10
const MB = 1 << 20
const GB = 1 << 30

func BytesSize(size int) string {
	if size > GB {
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	}
	if size > MB {
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	}
	if size > KB {
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	}
	return fmt.Sprintf("%.2f B", float64(size))
}

func ProcessMediaFile(app *ProcessorData, mediaFile *MediaFile, ui UI) {
	if mediaFile.Stage != Waiting {
		return
	}
	if mediaFile.Type == UnknownType {
		return
	}

	inputPath := path.Join(mediaFile.Dir, mediaFile.Name)

	inputFileInfo, err := os.Stat(inputPath)
	if err != nil {
		mediaFile.Error = fmt.Errorf("Can't find input file: %w", err)
		log.Println(mediaFile.Error)
		return
	}

	outputPath := path.Join(app.DstDir, mediaFile.Name)
	tempPath := path.Join(app.TmpDir, mediaFile.Name)

	mediaFile.Stage = ProcessingInProgress
	var result error

	request := ProcessingRequest{
		Target: mediaFile,
		InputPath: inputPath,
		OutputPath: tempPath,
	}

	switch mediaFile.Type {
		case Video:
			result = ShrinkMovie(request, ui)
		case JPG:
			result = ShrinkJPG(request)
		case PNG:
			result = ShrinkPNG(request)
		default:
			result = fmt.Errorf("*** ERROR: unsupported media type:", mediaFile.Type)
	}

	ui.Update()

	if result != nil {
		ui.LogError("%+v", result)
		mediaFile.Stage = ProcessingError
		mediaFile.Error = result
		// FIXME: should we clean up or keep the file so the user can inspect it?
		// os.Remove(tempPath)
		return
	}

	tempFileInfo, err := os.Stat(tempPath)
	if err != nil {
		mediaFile.Error = fmt.Errorf("Can't find output file: %w", err)
		log.Println(mediaFile.Error)
		return
	}

	mediaFile.Stage = ProcessingSuccess
	mediaFile.Error = nil

	var renameError error

	if int(tempFileInfo.Size()) > int(inputFileInfo.Size()) {
		ui.LogError("Converted file (%s) is bigger than input file (%s)! using input file", BytesSize(int(tempFileInfo.Size())), BytesSize(int(inputFileInfo.Size())))
		renameError = copyFile(inputPath, outputPath)
	} else {
		renameError = os.Rename(tempPath, outputPath)
	}

	if renameError != nil {
		mediaFile.Error = fmt.Errorf("Conversion failed; final rename step failed: %w", err)
		log.Println(mediaFile.Error)
		return
	}

	// check the file was written properly or not
	outFileInfo, err := os.Stat(outputPath)
	if err != nil {
		mediaFile.Error = fmt.Errorf("could not confirm output file written: %w", err)
		log.Println(mediaFile.Error)
		return
	}

	// Set the modified timestamp the same as the input file to preserve movie/image creation date
	os.Chtimes(outputPath, inputFileInfo.ModTime(), inputFileInfo.ModTime())

	mediaFile.ShrunkSize = int(outFileInfo.Size())
	ui.Update()
}

func (stats *ShrunkStats) ShrunkString() string {
	percentage := float64(stats.SizeAfter)/float64(stats.SizeBefore) * 100
	return fmt.Sprintf("Shrunk %d files [%s] -> [%s] (%.2f%%)", stats.Count, BytesSize(stats.SizeBefore), BytesSize(stats.SizeAfter), percentage)
}

func (stats *ShrunkStats) CleanedString() string {
	return fmt.Sprintf("Deleted %d files [%s]. Space opened up after shrinking: [%s]", stats.DeletedCount, BytesSize(stats.DeletedSize), BytesSize(stats.DeletedSize - stats.DeletedShrunkSize))
}


func AccumelateStats(files []MediaFile) (stats ShrunkStats) {
	for index := range files {
		mediaFile := &files[index]
		if mediaFile.ShrunkSize > 0 && mediaFile.Error == nil {
			stats.Count += 1
			stats.SizeBefore += mediaFile.Size
			stats.SizeAfter += mediaFile.ShrunkSize
			if mediaFile.Deleted {
				stats.DeletedCount += 1
				stats.DeletedSize += mediaFile.Size
				stats.DeletedShrunkSize += mediaFile.ShrunkSize
			}
		}
	}
	return
}

func InitProcessorData(opts Options) *ProcessorData {
	srcFiles, err := ListMediaFiles(opts.SrcDir)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	// make sure destination and temporary directories exist
	os.MkdirAll(opts.DstDir, 0o755)
	os.MkdirAll(opts.TmpDir, 0o755)

	// Find out which files are already processed
	{
		dstFiles, err := ListMediaFiles(opts.DstDir)
		if err != nil {
			log.Println(err)
		}

		for index := range srcFiles {
			srcEntry := &srcFiles[index]
			// find a shrunk version of the file
			for _, dstEntry := range dstFiles {
				if dstEntry.Name == srcEntry.Name {
					srcEntry.Stage = AlreadyProcessed
					srcEntry.ShrunkSize = dstEntry.Size
					break
				}
			}
		}
	}

	// Sort by name
	// FIXME allow the user to choose sorting method
	sort.Slice(srcFiles, func (i, j int) bool {
		return srcFiles[i].Name < srcFiles[j].Name
	});

	return &ProcessorData {
		Options: opts,
		MediaFiles: srcFiles,
	}
}

func StartProcessing(proc *ProcessorData, ui UI) {
	// TODO: process pictures first since they are much faster to process

	srcFiles := proc.MediaFiles

	// print current situation
	for index := range srcFiles {
		mediaFile := &srcFiles[index]
		fmt.Println(fileStats("File", mediaFile))
	}
	stats0 := AccumelateStats(srcFiles)
	if stats0.Count > 0 {
		fmt.Println(stats0.ShrunkString())
	}

	// Delete files that are done!
	// Do this before other tasks ..
	if proc.Options.DoClean {
		for index := range srcFiles {
			mediaFile := &srcFiles[index]
			if mediaFile.Stage == AlreadyProcessed && !mediaFile.Deleted {
				removeMediaFile(mediaFile, ui)
			}
		}
	}

	// maybe we should change the flag name?
	if !proc.Options.ReportOnly { // if we want to actually process, start the processing loop!
		for index := range srcFiles {
			proc.CurrentIndex = index
			mediaFile := &srcFiles[index]
			if proc.Options.DoClean && mediaFile.Stage == AlreadyProcessed && !mediaFile.Deleted {
				removeMediaFile(mediaFile, ui)
			}
			if mediaFile.Stage != Waiting {
				continue
			}
			mediaFile.StartTime = time.Now()
			ui.LogError("Shrinking %s [%s]\n", mediaFile.Name, BytesSize(mediaFile.Size))
			ProcessMediaFile(proc, mediaFile, ui)
			mediaFile.EndTime = time.Now()
			if mediaFile.Error == nil && mediaFile.Stage == ProcessingSuccess {
				ui.LogError(fileStats("Shrunk", mediaFile))
				if proc.Options.DoClean {
					removeMediaFile(mediaFile, ui)
				}
			}
		}
	}

	stats1 := AccumelateStats(srcFiles)
	if stats1.Count > stats0.Count {
		ui.LogError(stats1.ShrunkString())
	}
	if stats1.DeletedCount > 0 {
		ui.LogError(stats1.CleanedString())
	}
}

func removeMediaFile(mediaFile *MediaFile, ui UI) error {
	inputPath := path.Join(mediaFile.Dir, mediaFile.Name)
	err := os.Remove(inputPath)
	if err == nil {
		log.Println("Deleted file", mediaFile.Name)
		mediaFile.Deleted = true
	}
	ui.Update()
	return err
}

func fileStats(prefix string, mediaFile *MediaFile) string {
	if mediaFile.ShrunkSize == 0 {
		return fmt.Sprintf("%s %s [%s]", prefix, mediaFile.Name, BytesSize(mediaFile.Size))
	} else {
		percentage := float64(mediaFile.ShrunkSize)/float64(mediaFile.Size) * 100
		return fmt.Sprintf("%s %s [%s] -> [%s] (%.2f%%)", prefix, mediaFile.Name, BytesSize(mediaFile.Size), BytesSize(mediaFile.ShrunkSize), percentage)
	}
}
