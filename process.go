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

	mediaFile.Stage = ProcessingSuccess
	mediaFile.Error = nil
	err := os.Rename(tempPath, outputPath)
	if err != nil {
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

	mediaFile.ShrunkSize = int(outFileInfo.Size())
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

	// print current situation
	for _, mediaFile := range srcFiles {
		fmt.Printf("File: %s [%s]", mediaFile.Name, BytesSize(mediaFile.Size))
		if mediaFile.ShrunkSize > 0 {
			fmt.Printf(" -> [%s]", BytesSize(mediaFile.ShrunkSize))
		}
		fmt.Println()
	}

	// Process smaller files first
	// FIXME allow the user to choose sorting method
	sort.Slice(srcFiles, func (i, j int) bool {
		return srcFiles[i].Size < srcFiles[j].Size
	});

	for index := range srcFiles {
		mediaFile := &srcFiles[index]
		if app.DoClean && mediaFile.Stage == AlreadyProcessed {
			removeMediaFile(mediaFile)
		}
		if mediaFile.Stage != Waiting {
			continue
		}
		log.Printf("Shrinking %s [%s]\n", mediaFile.Name, BytesSize(mediaFile.Size))
		ProcessMediaFile(app, mediaFile)
		if mediaFile.Error == nil && mediaFile.Stage == ProcessingSuccess {
			log.Printf("Shrunk %s [%s] -> [%s]\n", mediaFile.Name, BytesSize(mediaFile.Size), BytesSize(mediaFile.ShrunkSize))
			if app.DoClean {
				removeMediaFile(mediaFile)
			}
		}
	}
	fmt.Println("Done")
}

func removeMediaFile(mediaFile *MediaFile) error {
	inputPath := path.Join(mediaFile.Dir, mediaFile.Name)
	log.Println("Removing input file")
	log.Println("    $ rm", inputPath)
	return os.Remove(inputPath)

}
