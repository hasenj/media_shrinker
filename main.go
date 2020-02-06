package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"flag"
	"io"
	"bufio"
	"strings"
	"time"
	"math"
)

type VideoFile struct {
	Dir, Name string
	Size int // in bytes
}

type VideoFileStatus struct {
	Name string
	Size int

	IsShrunk bool
	ShrunkSize int

	HadError bool // flag to avoid trying to process again
}

func ListVideos(dir string) []VideoFile {
	f, err := os.Open(dir)
	if err != nil {
		fmt.Println("Error reading directory", dir, err)
		os.Exit(1)
		return nil
	}
	entries, err := f.Readdir(-1)
	if err != nil {
		fmt.Println("Error listing directory", dir, err)
		os.Exit(1)
		return nil
	}
	files := make([]VideoFile, 0)
	for _, fentry := range entries {
		name := fentry.Name()
		ext := path.Ext(name)
		if ext != ".mp4" { // TODO: support more extensions!
			continue
		}
		size := int(fentry.Size())
		if size < MB {
			continue
		}
		vfile := VideoFile {
			Dir: dir,
			Name: name,
			Size: int(fentry.Size()),
		}
		files = append(files, vfile)
	}
	return files
}


const KB = 1 << 10
const MB = 1 << 20
const GB = 1 << 30

func BytesSize(size int) string {
	if size > GB {
		return fmt.Sprintf("%.2f GB", float64(size) / GB)
	}
	if size > MB {
		return fmt.Sprintf("%.2f MB", float64(size) / MB)
	}
	if size > KB {
		return fmt.Sprintf("%.2f KB", float64(size) / KB)
	}
	return fmt.Sprintf("%.2f B", float64(size))
}

// outputs the number of seconds.
func ParseTime(ts string) float64 {
	var hours, minutes, seconds, ss int
	fmt.Sscanf(ts, "%d:%d:%d.%d", &hours, &minutes, &seconds, &ss)
	// NOTE: assuming ss is always just two digits
	return (float64(ss) / 100.0) + float64(seconds + minutes * 60 + hours * 3600)
}

func FormatTime(s float64) string {
	seconds := int(s) % 60
	minutes := int(s / 60) % 60
	hours := int(s / 3600)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

type VideoSize struct {
	Width int
	Height int
	Duration float64
}

func DurationsEqual(dur1, dur2 float64) bool {
	return math.Abs(dur1 - dur2) < 0.1
}

func ProbeSize(inpath string) (out VideoSize, err error) {
	// get the video's dimensions
	//
	//    ffprobe -v error -select_streams v:0 -show_entries stream=width,height,duration -of default=noprint_wrappers=1:nokey=1 VID_20191207_115139.mp4
	//    1920
	//    1080
	//    75.049911
	//
	var probeArgs = []string {
		"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inpath,
	}
	probeCmd := exec.Command("ffprobe", probeArgs...)
	output, err := probeCmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("Could not get video dimensions. ffprobe command failed with: %w", err)
	}

	_, err = fmt.Sscan(string(output), &out.Width, &out.Height, &out.Duration)
	if err != nil {
		return out, fmt.Errorf("Could not get video dimensions. ffprobe output parsing failed with: %w", err)
	}
	return out, nil
}

func (app *Converter) ShrinkMovie(movie *VideoFileStatus) error {
	inpath := path.Join(app.SrcDir, movie.Name)
	size, err := ProbeSize(inpath)
	if err != nil {
		return err
	}

	var desired_width = 1080
	if size.Width < size.Height { // vertical video
		desired_width = 720
	}

	// ffmpeg -i SRC/NAME -vf scale="DESIRED_WIDTH:-1" DST/NAME
	outpath := path.Join(app.DstDir, movie.Name)
	temppath := path.Join(app.TmpDir, movie.Name)
	var args = []string {
		"-y", "-i", inpath, "-vf", fmt.Sprintf(`scale=%d:-1`, desired_width), temppath,
	}
	cmd := exec.Command("ffmpeg", args...)

	cmdout, err := cmd.StderrPipe()
	if err != nil {
		panic(fmt.Errorf("programmer error: incorrect usage of command piping: %w", err))
	}

	startTime := time.Now()
	fmt.Println(cmd.String())
	cmd.Start()

	reader := bufio.NewReader(cmdout)
	for {
		line, err := reader.ReadString('\r')

		timestampIndex := strings.LastIndex(line, "time=")
		if timestampIndex == -1 {
			fmt.Println("warning: no timestamp found!!")
			continue // should not happen?!
		}
		timestampIndex += len("time=")
		ts := line[timestampIndex:]
		spaceIndex := strings.Index(ts, " ")
		if spaceIndex == -1 {
			fmt.Println("could not parse timestamp:", ts)
			continue
		}
		ts = ts[:spaceIndex]
		durationProcessed := ParseTime(ts) // of the video
		progress := (float64(durationProcessed) / size.Duration) * 100
		timePassed := time.Since(startTime) // monotonic clock time
		fmt.Printf("%s -> %.2f%% [%.2f / %.2f]     \r", FormatTime(timePassed.Seconds()), progress, durationProcessed, size.Duration)

		if err == io.EOF {
			break
		} else if err != nil {
			// unexpected error!! what could it be?!
			// TODO: return this error?
			fmt.Printf("I/O error while interacting with ffmpeg", err)
			break
		}
	}

	{
		err := cmd.Wait()
		if err != nil {
			return fmt.Errorf("ffmpeg did not close properly? %w", err)
		}
	}

	// check the duration of the written file to temppath matches our duration, and if so, move it to outpath
	outSize, err := ProbeSize(temppath)
	if err != nil {
		os.Remove(temppath)
		return fmt.Errorf("Conversion appears to be failed because ffprobe failed: %w", err)
	}

	if DurationsEqual(size.Duration, outSize.Duration) {
		os.Rename(temppath, outpath)
		fmt.Println("Wrote shrunk file to", outpath)
	} else {
		return fmt.Errorf("Conversion failed; duration mismatch: %8.2f -> %8.2f", size.Duration, outSize.Duration)
	}

	// chekc the file was written properly or not
	outFileInfo, err := os.Stat(outpath)
	if err != nil {
		return fmt.Errorf("could not confirm output file written: %w", err)
	}
	movie.IsShrunk = true
	movie.ShrunkSize = int(outFileInfo.Size())
	if app.DoClean {
		fmt.Println("Removing input file")
		fmt.Println("rm", inpath)
		os.Remove(inpath)
	}
	return nil
}

type Converter struct {
	SrcDir, DstDir, TmpDir string
	DoClean bool
	Command string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Must take at least a command: verify or convert")
		os.Exit(1)
	}
	cmd := os.Args[1]
	var app Converter
	var args []string
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}
	f := flag.NewFlagSet(cmd, flag.ExitOnError)
	f.StringVar(&app.SrcDir, "src", ".", "The directory with the source video files")
	f.StringVar(&app.DstDir, "dst", "./smaller", "The directory where compressed videos are to be placed")
	f.StringVar(&app.TmpDir, "tmp", "./_temp_", "The directory where compressed videos are to be placed")
	f.BoolVar(&app.DoClean, "clean", false, "For 'verify' mode, whether to clean verified converted files or not")
	f.Parse(args)

	srcMovies := ListVideos(app.SrcDir)
	dstMovies := ListVideos(app.DstDir)

	movies := make([]VideoFileStatus, 0)
	for _, srcEntry := range srcMovies {
		var movie VideoFileStatus
		movie.Name = srcEntry.Name
		movie.Size = srcEntry.Size

		// find a shrunk version of the movie
		for _, dstEntry := range dstMovies {
			if dstEntry.Name == srcEntry.Name {
				movie.IsShrunk = true
				movie.ShrunkSize = dstEntry.Size
				break
			}
		}

		movies = append(movies, movie)
	}

	switch cmd {
	case "verify":
		// verify each shrunk file that the durations match!
		for _, movie := range movies {
			if !movie.IsShrunk {
				continue
			}
			inpath := path.Join(app.SrcDir, movie.Name)
			outpath := path.Join(app.DstDir, movie.Name)
			inSize, err := ProbeSize(inpath)
			if err != nil {
				fmt.Println(err)
				continue
			}
			outSize, err := ProbeSize(outpath)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Printf("%s: %8.2f  -> %8.2f ", movie.Name, inSize.Duration, outSize.Duration)
			if DurationsEqual(inSize.Duration, outSize.Duration) {
				fmt.Println("        [OK]")
				if app.DoClean {
					fmt.Println("rm", inpath)
					os.Remove(inpath)
				}
			} else {
				fmt.Println("  [MISMATCH]")
			}
		}
	case "convert":
		// print current situation without verifying size
		for _, movie := range movies {
			fmt.Printf("File: %s [%s]", movie.Name, BytesSize(movie.Size))
			if movie.IsShrunk {
				fmt.Printf(" -> [%s]", BytesSize(movie.ShrunkSize))
			}
			fmt.Println()
		}

		// pick the biggest file progressively and process it
		for {
			fmt.Println("looking for largest unprocessed file")
			// TODO: allow the user to send a signal that this is the last file to process!
			var largest *VideoFileStatus
			for index := range movies {
				movie := &movies[index]
				if movie.IsShrunk {
					continue
				}
				if largest == nil {
					largest = movie
				} else {
					if movie.Size > largest.Size {
						largest = movie
					}
				}
			}
			if largest == nil {
				fmt.Println("It appears we have processed all the files!")
				break
			}
			fmt.Printf("Shrinking %s [%s]\n", largest.Name, BytesSize(largest.Size))
			err := app.ShrinkMovie(largest)
			if err != nil {
				largest.HadError = true
				fmt.Println("Warning: shrinking %s failed: %s", largest.Name, err)
			} else {
				fmt.Printf("Shrunk %s [%s] -> [%s]\n", largest.Name, BytesSize(largest.Size), BytesSize(largest.ShrunkSize))
			}

		}
	}
}
