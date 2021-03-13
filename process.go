package media_shrinker

import (
	"fmt"
	"log"
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

func ProcessMediaFile(app *Processor, mediaFile *MediaFile) {
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
		InputPath: inputPath,
		OutputPath: tempPath,
	}

	switch mediaFile.Type {
		case Video:
			result = ShrinkMovie(request)
		case JPG:
			result = ShrinkJPG(request)
		case PNG:
			result = ShrinkPNG(request)
		default:
			log.Println("*** ERROR: unsupported media type:", mediaFile.Type)
	}

	if result != nil {
		log.Println(result)
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
		log.Printf("Converted file (%s) is bigger than input file (%s)! using input file", BytesSize(int(tempFileInfo.Size())), BytesSize(int(inputFileInfo.Size())))
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
}

func (stats *ShrunkStats) accumelate(mediaFile *MediaFile) {
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

func (stats *ShrunkStats) ShrunkString() string {
	percentage := float64(stats.SizeAfter)/float64(stats.SizeBefore) * 100
	return fmt.Sprintf("Shrunk %d files [%s] -> [%s] (%.2f%%)", stats.Count, BytesSize(stats.SizeBefore), BytesSize(stats.SizeAfter), percentage)
}

func (stats *ShrunkStats) CleanedString() string {
	return fmt.Sprintf("Deleted %d files [%s]. Space opened up after shrinking: [%s]", stats.DeletedCount, BytesSize(stats.DeletedSize), BytesSize(stats.DeletedSize - stats.DeletedShrunkSize))
}


func AccumelateStats(files []MediaFile) (stats ShrunkStats) {
	for index := range files {
		stats.accumelate(&files[index])
	}
	return
}

func DoProcess(app *Processor) {
	srcFiles, err := ListMediaFiles(app.SrcDir)
	if err != nil {
		log.Println(err)
		return
	}

	// make sure destination and temporary directories exist
	os.MkdirAll(app.DstDir, 0o755)
	os.MkdirAll(app.TmpDir, 0o755)

	// Find out which files are already processed
	{
		dstFiles, err := ListMediaFiles(app.DstDir)
		if err != nil {
			log.Println(err)
			return
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
	if app.DoClean {
		for index := range srcFiles {
			mediaFile := &srcFiles[index]
			if mediaFile.Stage == AlreadyProcessed && !mediaFile.Deleted {
				removeMediaFile(mediaFile)
			}
		}
	}

	// maybe we should change the flag name?
	if !app.ReportOnly { // if we want to actually process, start the processing loop!
		for index := range srcFiles {
			mediaFile := &srcFiles[index]
			if app.DoClean && mediaFile.Stage == AlreadyProcessed && !mediaFile.Deleted {
				removeMediaFile(mediaFile)
			}
			if mediaFile.Stage != Waiting {
				continue
			}
			log.Printf("Shrinking %s [%s]\n", mediaFile.Name, BytesSize(mediaFile.Size))
			ProcessMediaFile(app, mediaFile)
			if mediaFile.Error == nil && mediaFile.Stage == ProcessingSuccess {
				fmt.Println(fileStats("Shrunk", mediaFile))
				if app.DoClean {
					removeMediaFile(mediaFile)
				}
			}
		}
	}

	stats1 := AccumelateStats(srcFiles)
	if stats1.Count > stats0.Count {
		fmt.Println(stats1.ShrunkString())
	}
	if stats1.DeletedCount > 0 {
		fmt.Println(stats1.CleanedString())
	}

	fmt.Println("Done")
}

func removeMediaFile(mediaFile *MediaFile) error {
	inputPath := path.Join(mediaFile.Dir, mediaFile.Name)
	err := os.Remove(inputPath)
	if err == nil {
		log.Println("Deleted file", mediaFile.Name)
		mediaFile.Deleted = true
	}
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
