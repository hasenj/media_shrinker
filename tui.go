package media_shrinker

import "fmt"
import "os"
import "time"

import "github.com/gdamore/tcell/v2"
import "github.com/gdamore/tcell/v2/encoding"

import "github.com/mattn/go-runewidth"

// global - meant to be used by immediate drawing function to check mouse situation, etc
var CurrentEvent tcell.Event

var defStyle = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)

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

	s.SetStyle(defStyle)

	tui.Screen = s
	tui.Screen.EnableMouse()
}

func (tui *Tui) Loop() {
	s := tui.Screen
	for {
		CurrentEvent = s.PollEvent()
		switch ev := CurrentEvent.(type) {
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
				tui.filesView.ScrollUp()
				tui.Render()
			}
			if btns & tcell.WheelDown != 0 {
				tui.filesView.ScrollDown()
				tui.Render()
			}
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				s.Fini()
				killChildCommands()
				os.Exit(0)
			}
			if ev.Key() == tcell.KeyUp {
				tui.filesView.ScrollUp()
				tui.Render()
			}
			if ev.Key() == tcell.KeyDown {
				tui.filesView.ScrollDown()
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

	screenRect := tui.Rect
	screenRect.Width -= 1 // scrollbar
	filesRect, _ := SplitRectVInterpolate(screenRect, 0.7)
	{
		tui.filesView.Rect = filesRect
		screen := DrawScrollArea(&tui.filesView, tui.Screen)


		y := -tui.filesView.ScrollPosition

		// FIXME set these up during init
		textStyle := defStyle
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
				emitStr(screen, x0, y, waitingStyle, mediaFile.Name)
				y++
			case ProcessingError:
				emitStr(screen, x0, y, errorStyle, mediaFile.Name)
				y++
			case ProcessingSuccess, AlreadyProcessed:
				percentage := float64(mediaFile.ShrunkSize)/float64(mediaFile.Size) * 100

				emitStr(screen, x0, y, okStyle, mediaFile.Name)
				x := x0 + maxFileNameLength + 5
				x = emitStr(screen, x, y, tcell.StyleDefault, "[%s] -> [%s] (%.2f%%)", BytesSize(mediaFile.Size), BytesSize(mediaFile.ShrunkSize), percentage)
				if (mediaFile.Deleted) {
					emitStr(screen, x + 2, y, errorStyle, "DELETED")
				}
				y++
			case ProcessingInProgress:
				emitStr(screen, x0, y, activeStyle, mediaFile.Name)
				if mediaFile.Type == Video {
					// TODO show a progress bar
					// fmt.Printf("%s -> %.2f%% [%.2f / %.2f]        \r", FormatTime(timePassed.Seconds()), percentage, durationProcessed, size.Duration)
					x := x0 + maxFileNameLength + 5
					emitStr(screen, x, y, tcell.StyleDefault, "%.2f%%", mediaFile.Percentage)
					timePassed := FormatTime(time.Since(mediaFile.StartTime).Seconds())
					screenWidth, _ := screen.Size()
					emitStr(screen, screenWidth - 1 - len(timePassed), y, tcell.StyleDefault, timePassed)
				}
				y++
			}
		}
		tui.filesView.ScrollHeight = len(proc.MediaFiles) * 2
	}
}

func (area *TuiScrollArea) ScrollUp() {
	area.ScrollPosition--
	// minScrollPosition := area.Height - area.ScrollHeight
	minScrollPosition := 0
	if area.ScrollPosition < minScrollPosition {
		area.ScrollPosition = minScrollPosition
	}
}

func (area *TuiScrollArea) ScrollDown() {
	area.ScrollPosition++
	if area.ScrollPosition > area.ScrollHeight - 1 {
		area.ScrollPosition = area.ScrollHeight - 1
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

type ClippedScreen struct {
	tcell.Screen
	Rect // clipping rect
}

func (s *ClippedScreen) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < s.X {
		return
	}
	if x > s.X + s.Width {
		return
	}
	if y < s.Y {
		return
	}
	if y > s.Y + s.Height {
		return
	}
	s.Screen.SetContent(x, y, mainc, combc, style)
}

func (s *ClippedScreen) Size() (int, int) {
	return s.Rect.Width, s.Rect.Height
}

func MakeClippedScreen(screen tcell.Screen, rect Rect) *ClippedScreen {
	return &ClippedScreen { screen, rect }
}

type ViewPort struct {
	tcell.Screen
	Rect // clipping rect
}

func (s *ViewPort) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 {
		return
	}
	if x > s.Width {
		return
	}
	if y < 0 {
		return
	}
	if y > s.Height {
		return
	}
	s.Screen.SetContent(x + s.X, y + s.Y, mainc, combc, style)
}

func (s *ViewPort) Size() (int, int) {
	return s.Rect.Width, s.Rect.Height
}

func MakeViewPort(screen tcell.Screen, rect Rect) *ViewPort {
	return &ViewPort { screen, rect }
}

// --------

func DrawScrollArea(area *TuiScrollArea, screen tcell.Screen) tcell.Screen {
	textStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	rectStyle := textStyle.Dim(true)

	// TODO: add mouse wheel event handling and check if the mouse is within the area
	/*
		case *tcell.EventMouse:
			btns := ev.Buttons()
			if btns & tcell.WheelUp != 0 {
				tui.ScrollUp()
				tui.Render()
			}
			if btns & tcell.WheelDown != 0 {
				tui.ScrollDown()
				tui.Render()
			}
	*/

	// draw corners
	screen.SetContent(area.X, 			   area.Y,               tcell.RuneULCorner, nil, rectStyle)
	screen.SetContent(area.X + area.Width, area.Y,               tcell.RuneURCorner, nil, rectStyle)
	screen.SetContent(area.X,              area.Y + area.Height, tcell.RuneLLCorner, nil, rectStyle)
	screen.SetContent(area.X + area.Width, area.Y + area.Height, tcell.RuneLRCorner, nil, rectStyle)

	// draw borders
	for x := area.X + 1; x < area.X + area.Width ; x++ {
		screen.SetContent(x, area.Y,               tcell.RuneHLine, nil, rectStyle)
		screen.SetContent(x, area.Y + area.Height, tcell.RuneHLine, nil, rectStyle)
	}
	for y := area.Y + 1; y < area.Y + area.Height; y++ {
		screen.SetContent(area.X,              y, tcell.RuneVLine, nil, rectStyle)
		screen.SetContent(area.X + area.Width, y, tcell.RuneVLine, nil, rectStyle)
	}

	// draw the scroll thumb
	scrollbarHeight := area.Height - 2

	thumbPosition := int((float64(area.ScrollPosition) / float64(area.ScrollHeight)) * float64(scrollbarHeight))

	screen.SetContent(area.X + area.Width, area.Y + 1 + thumbPosition, tcell.RuneCkBoard, nil, rectStyle)

	innerRect := area.Rect
	innerRect.X += 1
	innerRect.Y += 1
	innerRect.Width -= 2
	innerRect.Height -= 2

	return MakeViewPort(screen, innerRect)
}

func SplitRectV(rect Rect, at int) (top Rect, bottom Rect) {
	top = rect
	bottom = rect

	top.Height = at - 1

	bottom.Y = at + 1
	bottom.Height -= (at - 1)

	return top, bottom
}

func SplitRectVInterpolate(rect Rect, at float64) (top Rect, bottom Rect) {
	return SplitRectV(rect, int(float64(rect.Height) * at))
}

func SplitRectH(rect Rect, at int) (left Rect, right Rect) {
	left = rect
	right = rect
	left.Width = at - 1
	left.X = at + 1
	left.Width -= (at - 1)

	return left, right
}
