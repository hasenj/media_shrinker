package media_shrinker

import "fmt"
import "os"
import "time"

import "github.com/gdamore/tcell/v2"
import "github.com/gdamore/tcell/v2/encoding"

import "github.com/mattn/go-runewidth"

// global - meant to be used by immediate drawing function to check mouse situation, etc
var CurrentEvent tcell.Event

// box drawing
const LineH = '━'
const LineV = '┃'
const TreeR = '┣'
const TreeL = '┫'
const TreeT = '┳'
const TreeB = '┻'
const Cross = '╋'

// var lg = log.New(os.Stderr, "", 0)

var defStyle = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
var rectStyle = defStyle.Dim(true)
var textStyle = defStyle
var waitingStyle = textStyle.Dim(true)
var errorStyle = textStyle.Foreground(tcell.ColorDarkRed)
var okStyle = textStyle.Foreground(tcell.ColorForestGreen)
var activeStyle = textStyle.Foreground(tcell.ColorGreen)

func (tui *Tui) Init() {
	encoding.Register()

	s, e := tcell.NewScreen()
	if e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
	if e := s.Init(); e != nil {
		fmt.Println(e)
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
			tui.Render()
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

	screenViewPort := AsViewPort(tui.Screen)
	filesViewPort, messagesViewPort := screenViewPort.SplitVf(0.7)
	{
		view := &tui.filesView
		view.ScrollHeight = len(proc.MediaFiles) * 2
		viewport := IMScrollArea(view, filesViewPort)

		y := -view.ScrollPosition

		const x0 = 4
		for index := range proc.MediaFiles {
			y++
			mediaFile := &proc.MediaFiles[index]
			switch mediaFile.Stage {
			case Waiting:
				Print(viewport, x0, y, waitingStyle, mediaFile.Name)
				y++
			case ProcessingError:
				Print(viewport, x0, y, errorStyle, mediaFile.Name)
				y++
			case ProcessingSuccess, AlreadyProcessed:
				percentage := float64(mediaFile.ShrunkSize)/float64(mediaFile.Size) * 100

				Print(viewport, x0, y, okStyle, mediaFile.Name)
				x := x0 + maxFileNameLength + 5
				x = Printf(viewport, x, y, tcell.StyleDefault, "[%s] -> [%s] (%.2f%%)", BytesSize(mediaFile.Size), BytesSize(mediaFile.ShrunkSize), percentage)
				if (mediaFile.Deleted) {
					Print(viewport, x + 2, y, errorStyle, "DELETED")
				}
				y++
			case ProcessingInProgress:
				Print(viewport, x0, y, activeStyle, mediaFile.Name)
				if mediaFile.Type == Video {
					// TODO show a progress bar
					// fmt.Printf("%s -> %.2f%% [%.2f / %.2f]        \r", FormatTime(timePassed.Seconds()), percentage, durationProcessed, size.Duration)
					x := x0 + maxFileNameLength + 5
					Printf(viewport, x, y, tcell.StyleDefault, "%.2f%%", mediaFile.Percentage)
					timePassed := FormatTime(time.Since(mediaFile.StartTime).Seconds())
					Print(viewport, viewport.Width - 1 - len(timePassed), y, tcell.StyleDefault, timePassed)
				}
				y++
			}
		}
	}
	{
		view := &tui.messagesView
		view.ScrollHeight = len(tui.Messages)

		viewport := IMScrollArea(view, messagesViewPort)

		y := -view.ScrollPosition
		for _, message := range tui.Messages {
			if y >= 0 {
				Print(viewport, 0, y, defStyle, message)
			}
			if y >= viewport.Height {
				break
			}
			y++
		}
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

func (tui *Tui) Logf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	tui.Messages = append(tui.Messages, message)
}

func (tui *Tui) Log(message string) {
	tui.Messages = append(tui.Messages, message)
}

func Printf(s tcell.Screen, x, y int, style tcell.Style, format string, a ...interface{}) int {
	message := fmt.Sprintf(format, a...)
	return Print(s, x, y, style, message)
}

func Print(s tcell.Screen, x, y int, style tcell.Style, message string) int {
	width, height := s.Size()
	if !(y >= 0 && y < height) {
		return x
	}
	for _, c := range message {
		if x > width {
			break;
		}
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

func (s *ViewPort) SetContent(x int, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 {
		return
	}
	if x >= s.Width {
		return
	}
	if y < 0 {
		return
	}
	if y >= s.Height {
		return
	}
	s.Screen.SetContent(x + s.X, y + s.Y, mainc, combc, style)
}

func (s *ViewPort) GetContent(x int, y int) (rune, []rune, tcell.Style, int) {
	return s.Screen.GetContent(x + s.X, y + s.Y)
}

func (s *ViewPort) GetRune(x int, y int) rune {
	r, _, _, _ := s.GetContent(x, y)
	return r
}

func (s *ViewPort) Size() (int, int) {
	return s.Rect.Width, s.Rect.Height
}

func MakeViewPort(screen tcell.Screen, rect Rect) *ViewPort {
	return &ViewPort { screen, rect }
}

func AsViewPort(screen tcell.Screen) *ViewPort {
	var rect Rect
	rect.Width, rect.Height = screen.Size()
	return &ViewPort { screen, rect }
}

// --------

func MousePosition(ev *tcell.EventMouse) Point {
	x, y := ev.Position()
	return Point { X: x, Y: y }
}

// Immediate Mode Scroll Area
// Draws scrollbar and handles mouse wheel events
// Returns the viewport within (scrollbar space removed)
func IMScrollArea(area *TuiScrollArea, vp *ViewPort) *ViewPort {
	//  mouse wheel scrolling
	switch ev := CurrentEvent.(type) {
		case *tcell.EventMouse:
			// this assumes that area.Rect is the real rect not a translated one ..
			if RectContains(vp.Rect, MousePosition(ev)) {
				btns := ev.Buttons()
				if btns & tcell.WheelUp != 0 {
					area.ScrollUp()
				}
				if btns & tcell.WheelDown != 0 {
					area.ScrollDown()
				}
			}
	}

	// if no need for a scrollbar, don't draw it
	// and just return the original viewport as the inner one
	if area.ScrollHeight <= vp.Height {
		return vp
	}

	const ScrollBarBG = ' ' // or '░' // or '▒'
	const ScrollBarFG = '▉' // or '▓'

	scrollBarStyle := rectStyle.Background(tcell.ColorGrey)

	// Draw a vertical line
	x := vp.Width - 1
	for y := 0; y < vp.Height; y++ {
		vp.SetContent(x, y, ScrollBarBG, nil, scrollBarStyle)
	}

	// draw the scroll thumb
	scrollbarHeight := vp.Height

	if area.ScrollHeight > 0 {
		thumbY := int((float64(area.ScrollPosition) / float64(area.ScrollHeight)) * float64(scrollbarHeight))
		vp.SetContent(x, thumbY, ScrollBarFG, nil, scrollBarStyle)
	}

	var innerRect Rect = vp.Rect
	innerRect.Width -= 1
	return MakeViewPort(vp.Screen, innerRect)
}

func (v *ViewPort) SplitV(at int) (top *ViewPort, bottom *ViewPort) {
	rect := v.Rect
	topR := rect
	bottomR := rect

	topR.Height = at

	bottomR.Y = at + 1
	bottomR.Height = rect.Height - at - 1

	top = MakeViewPort(v, topR)
	bottom = MakeViewPort(v, bottomR)


	// Draw the horizontal line
	// First we need to decide for each edge whether to draw just the line or its intersection with another line
	{
		exLeft  := v.GetRune(0,           at)
		exRight := v.GetRune(v.Width - 1, at)

		var rLeft  rune = LineH
		var rRight rune = LineH

		switch exLeft {
		case LineV:
			rLeft = TreeR
		case TreeL:
			rLeft = Cross
		}
		switch exRight {
		case LineV:
			rRight = TreeL
		case TreeR:
			rRight = Cross
		}

		v.SetContent(0,       at, rLeft,  nil, rectStyle)
		v.SetContent(v.Width - 1, at, rRight, nil, rectStyle)

		for i := 1; i < v.Width - 1; i++ {
			v.SetContent(i, at, LineH, nil, rectStyle)
		}
	}

	return
}

func (v *ViewPort) SplitVf(at float64) (top *ViewPort, bottom *ViewPort) {
	return v.SplitV(int(float64(v.Rect.Height) * at))
}

func SplitRectH(rect Rect, at int) (left Rect, right Rect) {
	left = rect
	right = rect
	left.Width = at - 1
	left.X = at + 1
	left.Width -= (at - 1)

	return left, right
}

func RectContains(rect Rect, point Point) bool {
	return point.X >= rect.X && point.X < rect.X + rect.Width &&
		point.Y >= rect.Y && point.Y < rect.Y + rect.Height
}
