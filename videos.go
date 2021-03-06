package media_shrinker

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	// "time"
)

// ParseTime_FF parses the timestamp strings printed by ffmpeg during processing
// outputs the number of seconds.
func ParseTime_FF(ts string) float64 {
	var hours, minutes, seconds, ss int
	fmt.Sscanf(ts, "%d:%d:%d.%d", &hours, &minutes, &seconds, &ss)
	// NOTE: assuming ss is always just two digits
	return (float64(ss) / 100.0) + float64(seconds+minutes*60+hours*3600)
}

func FormatTime(s float64) string {
	seconds := int(s) % 60
	minutes := int(s/60) % 60
	hours := int(s / 3600)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

type VideoSize struct {
	Width    int
	Height   int

	// in seconds
	Duration float64
}

// DurationsRoughlyEqual allows a difference of about one second
func DurationsRoughlyEqual(dur1, dur2 float64) bool {
	return math.Abs(dur1-dur2) < 1
}

func ProbeVideoSize(inpath string) (out VideoSize, err error) {
	// get the video's dimensions
	//
	//    ffprobe -v fatal -select_streams v:0 -show_entries stream=width,height,duration -of default=noprint_wrappers=1:nokey=1 VID_20191207_115139.mp4
	//    1920
	//    1080
	//    75.049911
	//
	var probeArgs = []string{
		"-v", "fatal", "-select_streams", "v:0", "-show_entries", "stream=width,height,duration",
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

// returns nil if success
func ShrinkMovie(request ProcessingRequest, ui UI) (result error) {
	size, err := ProbeVideoSize(request.InputPath)
	if err != nil {
		return fmt.Errorf("Probing video size failed: %w", err)
	}

	// FIXME: maybe if size is already smaller than desired, don't scale up!!

	var desired_width = 1080
	if size.Width < size.Height { // vertical video
		desired_width = 720
	}

	// ffmpeg -i SRC/NAME -vf scale="DESIRED_WIDTH:-1" DST/NAME
	var args = []string{
		"-y", "-i", request.InputPath,
	}
	if size.Width > desired_width {
		args = append(args, "-vf", fmt.Sprintf(`scale=%d:-1`, desired_width))
	}
	args = append(args, "-c:v", "libx264", "-crf", "26")
	args = append(args, request.OutputPath)
	cmd := exec.Command("ffmpeg", args...)
	registerCommand(cmd)

	cmdout, err := cmd.StderrPipe()
	if err != nil {
		panic(fmt.Errorf("programmer error: incorrect usage of command piping: %w", err))
	}

	// startTime := time.Now()
	ui.Log(cmd.String())
	cmd.Start()

	// Read the text output of ffmpeg and parse it to understand progress and present it to the user
	reader := bufio.NewReader(cmdout)
	for {
		line, err := reader.ReadString('\r')

		if err != nil {
			if err != io.EOF {
				// unexpected error!! what could it be?!
				// This is an IO error. It doesn't necessarily mean processing failed.
				// Just break out of the I/O parsing loop and
				// wait for the FFMPEG process to finish
				ui.Logf("I/O error while interacting with ffmpeg %w", err)

				// FIXME end the process now and return the error!!
			}
			break
		}

		timestampIndex := strings.LastIndex(line, "time=")
		if timestampIndex == -1 {
			ui.Log("warning: no timestamp found!!")
			continue // should not happen?!
		}
		timestampIndex += len("time=")
		ts := line[timestampIndex:]
		spaceIndex := strings.Index(ts, " ")
		if spaceIndex == -1 {
			ui.Logf("could not parse timestamp: %v", ts)
			continue
		}
		ts = ts[:spaceIndex]
		durationProcessed := ParseTime_FF(ts) // of the video
		percentage := (float64(durationProcessed) / size.Duration) * 100
		// timePassed := time.Since(startTime) // monotonic clock time
		request.Target.Percentage = percentage
		request.Target.Processed = durationProcessed
		ui.Update()

		// FIXME use a "print status line" function instead?
		// fmt.Printf("%s -> %.2f%% [%.2f / %.2f]        \r", FormatTime(timePassed.Seconds()), percentage, durationProcessed, size.Duration)
	}

	// Wait for ffmpeg process to finish
	{
		err := cmd.Wait()
		if err != nil {
			return fmt.Errorf("ffmpeg did not close properly? %w", err)
		}
	}

	// check the duration of the written file matches our duration
	{
		outSize, err := ProbeVideoSize(request.OutputPath)
		if err != nil {
			// os.Remove(request.OutputPath)
			return fmt.Errorf("Conversion appears to be failed because ffprobe failed: %w", err)
		}

		if !DurationsRoughlyEqual(size.Duration, outSize.Duration) {
			return fmt.Errorf("Conversion failed; duration mismatch: %8.2f -> %8.2f", size.Duration, outSize.Duration)
		}
	}

	// success!!
	return nil
}

var runningCommands []*exec.Cmd
func registerCommand(cmd *exec.Cmd) {
	runningCommands = append(runningCommands, cmd)
}

func killChildCommands() {
	for _, cmd := range runningCommands {
		// can fail silently if already killed - we don't care
		cmd.Process.Kill()
	}
}
