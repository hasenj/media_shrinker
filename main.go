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
)

type VideoFile struct {
	Dir, Name string
	Size int // in bytes
}

type VideoFileStatus struct {
	Name string
	Size int
	Src, Dst string

	IsShrunk bool
	ShrunkSize int

	HadError bool // flag to avoid trying to process again
}

func ListVideos(dir string) []VideoFile {
	f, err := os.Open(dir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		os.Exit(1)
		return nil
	}
	entries, err := f.Readdir(-1)
	if err != nil {
		fmt.Println("Error listing directory:", err)
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
func ParseTime(ts string) int {
	var hours, minutes, seconds, ss int
	fmt.Sscanf(ts, "%d:%d:%d.%d", &hours, &minutes, &seconds, &ss)
	return seconds + minutes * 60 + hours * 3600
}

func FormatTime(s float64) string {
	seconds := int(s) % 60
	minutes := int(s / 60) % 60
	hours := int(s / 3600)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func ShrinkMovie(movie *VideoFileStatus) error {
	// get the video's dimensions
	//
	//    ffprobe -v error -select_streams v:0 -show_entries stream=width,height,duration -of default=noprint_wrappers=1:nokey=1 VID_20191207_115139.mp4
	//    1920
	//    1080
	//    75.049911
	//
	inpath := path.Join(movie.Src, movie.Name)
	var probeArgs = []string {
		"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inpath,
	}
	probeCmd := exec.Command("ffprobe", probeArgs...)
	output, err := probeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Could not get video dimensions. ffprobe command failed with: %w", err)
	}

	var width, height int
	var duration float64
	{
		_, err := fmt.Sscan(string(output), &width, &height, &duration)
		if err != nil {
			return fmt.Errorf("Could not get video dimensions. ffprobe output parsing failed with: %w", err)
		}
	}

	var desired_width = 1080
	if width < height { // vertical video
		desired_width = 720
	}

	// ffmpeg -i SRC/NAME -vf scale="DESIRED_WIDTH:-1" DST/NAME
	outpath := path.Join(movie.Dst, movie.Name)
	var args = []string {
		"-y", "-i", inpath, "-vf", fmt.Sprintf(`scale=%d:-1`, desired_width), outpath,
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
		progress := int((float64(durationProcessed) / duration) * 100)
		timePassed := time.Since(startTime) // monotonic clock time
		fmt.Printf("%s -> %d%% [%d / %d]     \r", FormatTime(timePassed.Seconds()), progress, durationProcessed, int(duration))

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

	// chekc the file was written properly or not
	outFileInfo, err := os.Stat(outpath)
	if err != nil {
		return fmt.Errorf("could not confirm output file written: %w", err)
	}
	movie.IsShrunk = true
	movie.ShrunkSize = int(outFileInfo.Size())
	return nil
}

func main() {
	var src, dst string
	flag.StringVar(&src, "src", ".", "The directory with the source video files")
	flag.StringVar(&dst, "dst", "./smaller", "The directory where compressed videos are to be placed")

	flag.Parse()

	srcMovies := ListVideos(src)
	dstMovies := ListVideos(dst)

	movies := make([]VideoFileStatus, 0)
	for _, srcEntry := range srcMovies {
		var movie VideoFileStatus
		movie.Name = srcEntry.Name
		movie.Size = srcEntry.Size
		movie.Src = src
		movie.Dst = dst

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


	for _, movie := range movies {
		fmt.Printf("File: %s %s", movie.Name, BytesSize(movie.Size))
		if movie.IsShrunk {
			fmt.Printf(" -> %s", BytesSize(movie.ShrunkSize))
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
		fmt.Println("Processing file:", largest.Name)
		err := ShrinkMovie(largest)
		if err != nil {
			largest.HadError = true
			fmt.Println("Warning: shrinking %s failed: %s", largest.Name, err)
		}
		// break // TEMP only do it once for now!
	}
}
