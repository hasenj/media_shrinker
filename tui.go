package media_shrinker

import "fmt"
import "os"
import "time"

import "github.com/gdamore/tcell/v2"
import "github.com/gdamore/tcell/v2/encoding"

import "github.com/mattn/go-runewidth"

func (tui *Tui) Init() {
	encoding.Register()

	s, e := tcell.NewScreen()
	if e != nil {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}
	if e := s.Init(); e != nil {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}

	defStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorWhite)
	s.SetStyle(defStyle)

	tui.Screen = s
	tui.Screen.EnableMouse()
}

func (tui *Tui) Loop() {
	s := tui.Screen
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventResize:
			tui.Width, tui.Height = ev.Size()
			s.Sync()
			tui.Render()
		case *UpdateEvent:
			tui.Render()
		case *tcell.EventMouse:
			// for now only respond to scrolling
			btns := ev.Buttons()
			if btns & tcell.WheelUp != 0 {
				tui.ScrollUp()
				tui.Render()
			}
			if btns & tcell.WheelDown != 0 {
				tui.ScrollDown()
				tui.Render()
			}
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				s.Fini()
				killChildCommands()
				os.Exit(0)
			}
			if ev.Key() == tcell.KeyUp {
				tui.ScrollUp()
				tui.Render()
			}
			if ev.Key() == tcell.KeyDown {
				tui.ScrollDown()
				tui.Render()
			}
		}
	}
}


func (tui *Tui) Update() {
	var event UpdateEvent
	event.SetEventNow()
	tui.Screen.PostEvent(&event)
}

func (tui *Tui) Render() {
	// figure out what is going on right now ...
	proc := tui.Processor

	// filesCount := 0
	// filesProcessed := 0
	// filesDeleted := 0
	// currentFileSize := 0
	// currentFileProgress := 0

	maxFileNameLength := 0

	for index := range proc.MediaFiles {
		mediaFile := &proc.MediaFiles[index]
		fileNameLength := len(mediaFile.Name)
		if fileNameLength > maxFileNameLength {
			maxFileNameLength = fileNameLength
		}
	}

	tui.Screen.Clear()
	defer tui.Screen.Show()

	y := -tui.ScrollPosition

	// FIXME set these up during init
	textStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	waitingStyle := textStyle.Dim(true)
	errorStyle := textStyle.Foreground(tcell.ColorDarkRed)
	okStyle := textStyle.Foreground(tcell.ColorForestGreen)
	activeStyle := textStyle.Foreground(tcell.ColorGreen)

	const x0 = 4
	for index := range proc.MediaFiles {
		y++
		mediaFile := &proc.MediaFiles[index]
		switch mediaFile.Stage {
		case Waiting:
			emitStr(tui.Screen, x0, y, waitingStyle, mediaFile.Name)
			y++
		case ProcessingError:
			emitStr(tui.Screen, x0, y, errorStyle, mediaFile.Name)
			y++
		case ProcessingSuccess, AlreadyProcessed:
			percentage := float64(mediaFile.ShrunkSize)/float64(mediaFile.Size) * 100

			emitStr(tui.Screen, x0, y, okStyle, mediaFile.Name)
			x := x0 + maxFileNameLength + 5
			x = emitStr(tui.Screen, x, y, tcell.StyleDefault, "[%s] -> [%s] (%.2f%%)", BytesSize(mediaFile.Size), BytesSize(mediaFile.ShrunkSize), percentage)
			if (mediaFile.Deleted) {
				emitStr(tui.Screen, x + 2, y, errorStyle, "DELETED")
			}
			y++
		case ProcessingInProgress:
			emitStr(tui.Screen, x0, y, activeStyle, mediaFile.Name)
			if mediaFile.Type == Video {
				// TODO show a progress bar
				// fmt.Printf("%s -> %.2f%% [%.2f / %.2f]        \r", FormatTime(timePassed.Seconds()), percentage, durationProcessed, size.Duration)
				x := x0 + maxFileNameLength + 5
				emitStr(tui.Screen, x, y, tcell.StyleDefault, "%.2f%%", mediaFile.Percentage)
				timePassed := FormatTime(time.Since(mediaFile.StartTime).Seconds())
				emitStr(tui.Screen, tui.Width - len(timePassed), y, tcell.StyleDefault, timePassed)
			}
			y++
		}
	}
	tui.ScrollHeight = len(proc.MediaFiles) * 5
}

func (tui *Tui) ScrollUp() {
	tui.ScrollPosition--
	// minScrollPosition := tui.Height - tui.ScrollHeight
	minScrollPosition := 0
	if tui.ScrollPosition < minScrollPosition {
		tui.ScrollPosition = minScrollPosition
	}
}

func (tui *Tui) ScrollDown() {
	tui.ScrollPosition++
	if tui.ScrollPosition > tui.ScrollHeight - 1 {
		tui.ScrollPosition = tui.ScrollHeight - 1
	}
}

func (tui *Tui) LogError(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	tui.Messages = append(tui.Messages, message)
}

func emitStr(s tcell.Screen, x, y int, style tcell.Style, format string, a ...interface{}) int {
	message := fmt.Sprintf(format, a...)
	for _, c := range message {
		var comb []rune
		w := runewidth.RuneWidth(c)
		if w == 0 {
			comb = []rune{c}
			c = ' '
			w = 1
		}
		s.SetContent(x, y, c, comb, style)
		x += w
	}
	return x
}

