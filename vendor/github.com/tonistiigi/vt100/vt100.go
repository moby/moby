// package vt100 implements a quick-and-dirty programmable ANSI terminal emulator.
//
// You could, for example, use it to run a program like nethack that expects
// a terminal as a subprocess. It tracks the position of the cursor,
// colors, and various other aspects of the terminal's state, and
// allows you to inspect them.
//
// We do very much mean the dirty part. It's not that we think it might have
// bugs. It's that we're SURE it does. Currently, we only handle raw mode, with no
// cooked mode features like scrolling. We also misinterpret some of the control
// codes, which may or may not matter for your purpose.
package vt100

import (
	"bytes"
	"fmt"
	"image/color"
	"sort"
	"strings"
)

type Intensity int

const (
	Normal Intensity = 0
	Bright           = 1
	Dim              = 2
	// TODO(jaguilar): Should this be in a subpackage, since the names are pretty collide-y?
)

var (
	// Technically RGBAs are supposed to be premultiplied. But CSS doesn't expect them
	// that way, so we won't do it in this file.
	DefaultColor = color.RGBA{0, 0, 0, 0}
	// Our black has 255 alpha, so it will compare negatively with DefaultColor.
	Black   = color.RGBA{0, 0, 0, 255}
	Red     = color.RGBA{255, 0, 0, 255}
	Green   = color.RGBA{0, 255, 0, 255}
	Yellow  = color.RGBA{255, 255, 0, 255}
	Blue    = color.RGBA{0, 0, 255, 255}
	Magenta = color.RGBA{255, 0, 255, 255}
	Cyan    = color.RGBA{0, 255, 255, 255}
	White   = color.RGBA{255, 255, 255, 255}
)

func (i Intensity) alpha() uint8 {
	switch i {
	case Bright:
		return 255
	case Normal:
		return 170
	case Dim:
		return 85
	default:
		return 170
	}
}

// Format represents the display format of text on a terminal.
type Format struct {
	// Fg is the foreground color.
	Fg color.RGBA
	// Bg is the background color.
	Bg color.RGBA
	// Intensity is the text intensity (bright, normal, dim).
	Intensity Intensity
	// Various text properties.
	Underscore, Conceal, Negative, Blink, Inverse bool
}

func toCss(c color.RGBA) string {
	return fmt.Sprintf("rgba(%d, %d, %d, %f)", c.R, c.G, c.B, float32(c.A)/255)
}

func (f Format) css() string {
	parts := make([]string, 0)
	fg, bg := f.Fg, f.Bg
	if f.Inverse {
		bg, fg = fg, bg
	}

	if f.Intensity != Normal {
		// Intensity only applies to the text -- i.e., the foreground.
		fg.A = f.Intensity.alpha()
	}

	if fg != DefaultColor {
		parts = append(parts, "color:"+toCss(fg))
	}
	if bg != DefaultColor {
		parts = append(parts, "background-color:"+toCss(bg))
	}
	if f.Underscore {
		parts = append(parts, "text-decoration:underline")
	}
	if f.Conceal {
		parts = append(parts, "display:none")
	}
	if f.Blink {
		parts = append(parts, "text-decoration:blink")
	}

	// We're not in performance sensitive code. Although this sort
	// isn't strictly necessary, it gives us the nice property that
	// the style of a particular set of attributes will always be
	// generated the same way. As a result, we can use the html
	// output in tests.
	sort.StringSlice(parts).Sort()

	return strings.Join(parts, ";")
}

// Cursor represents both the position and text type of the cursor.
type Cursor struct {
	// Y and X are the coordinates.
	Y, X int

	// F is the format that will be displayed.
	F Format
}

// VT100 represents a simplified, raw VT100 terminal.
type VT100 struct {
	// Height and Width are the dimensions of the terminal.
	Height, Width int

	// Content is the text in the terminal.
	Content [][]rune

	// Format is the display properties of each cell.
	Format [][]Format

	// Cursor is the current state of the cursor.
	Cursor Cursor

	// savedCursor is the state of the cursor last time save() was called.
	savedCursor Cursor

	unparsed []byte
}

// NewVT100 creates a new VT100 object with the specified dimensions. y and x
// must both be greater than zero.
//
// Each cell is set to contain a ' ' rune, and all formats are left as the
// default.
func NewVT100(y, x int) *VT100 {
	if y == 0 || x == 0 {
		panic(fmt.Errorf("invalid dim (%d, %d)", y, x))
	}

	v := &VT100{
		Height:  y,
		Width:   x,
		Content: make([][]rune, y),
		Format:  make([][]Format, y),
	}

	for row := 0; row < y; row++ {
		v.Content[row] = make([]rune, x)
		v.Format[row] = make([]Format, x)

		for col := 0; col < x; col++ {
			v.clear(row, col)
		}
	}
	return v
}

func (v *VT100) UsedHeight() int {
	count := 0
	for _, l := range v.Content {
		for _, r := range l {
			if r != ' ' {
				count++
				break
			}
		}
	}
	return count
}

func (v *VT100) Resize(y, x int) {
	// add some minimal defaults to handle zero and negative values
	if x < 6 {
		x = 6
	}
	if y < 1 {
		y = 1
	}
	if y > v.Height {
		n := y - v.Height
		for row := 0; row < n; row++ {
			v.Content = append(v.Content, make([]rune, v.Width))
			v.Format = append(v.Format, make([]Format, v.Width))
			for col := 0; col < v.Width; col++ {
				v.clear(v.Height+row, col)
			}
		}
		v.Height = y
	} else if y < v.Height {
		v.Content = v.Content[:y]
		v.Format = v.Format[:y]
		v.Height = y
	}

	if x > v.Width {
		for i := range v.Content {
			row := make([]rune, x)
			copy(row, v.Content[i])
			v.Content[i] = row
			format := make([]Format, x)
			copy(format, v.Format[i])
			v.Format[i] = format
			for j := v.Width; j < x; j++ {
				v.clear(i, j)
			}
		}
		v.Width = x
	} else if x < v.Width {
		for i := range v.Content {
			v.Content[i] = v.Content[i][:x]
			v.Format[i] = v.Format[i][:x]
		}
		v.Width = x
	}
}

func (v *VT100) Write(dt []byte) (int, error) {
	n := len(dt)
	if len(v.unparsed) > 0 {
		dt = append(v.unparsed, dt...) // this almost never happens
		v.unparsed = nil
	}
	buf := bytes.NewBuffer(dt)
	for {
		if buf.Len() == 0 {
			return n, nil
		}
		cmd, err := Decode(buf)
		if err != nil {
			if l := buf.Len(); l > 0 && l < 12 { // on small leftover handle unparsed, otherwise skip
				v.unparsed = buf.Bytes()
			}
			return n, nil
		}
		v.Process(cmd) // ignore error
	}
}

// Process handles a single ANSI terminal command, updating the terminal
// appropriately.
//
// One special kind of error that this can return is an UnsupportedError. It's
// probably best to check for these and skip, because they are likely recoverable.
// Support errors are exported as expvars, so it is possibly not necessary to log
// them. If you want to check what's failed, start a debug http server and examine
// the vt100-unsupported-commands field in /debug/vars.
func (v *VT100) Process(c Command) error {
	return c.display(v)
}

// HTML renders v as an HTML fragment. One idea for how to use this is to debug
// the current state of the screen reader.
func (v *VT100) HTML() string {
	var buf bytes.Buffer
	buf.WriteString(`<pre style="color:white;background-color:black;">`)

	// Iterate each row. When the css changes, close the previous span, and open
	// a new one. No need to close a span when the css is empty, we won't have
	// opened one in the past.
	var lastFormat Format
	for y, row := range v.Content {
		for x, r := range row {
			f := v.Format[y][x]
			if f != lastFormat {
				if lastFormat != (Format{}) {
					buf.WriteString("</span>")
				}
				if f != (Format{}) {
					buf.WriteString(`<span style="` + f.css() + `">`)
				}
				lastFormat = f
			}
			if s := maybeEscapeRune(r); s != "" {
				buf.WriteString(s)
			} else {
				buf.WriteRune(r)
			}
		}
		buf.WriteRune('\n')
	}
	buf.WriteString("</pre>")

	return buf.String()
}

// maybeEscapeRune potentially escapes a rune for display in an html document.
// It only escapes the things that html.EscapeString does, but it works without allocating
// a string to hold r. Returns an empty string if there is no need to escape.
func maybeEscapeRune(r rune) string {
	switch r {
	case '&':
		return "&amp;"
	case '\'':
		return "&#39;"
	case '<':
		return "&lt;"
	case '>':
		return "&gt;"
	case '"':
		return "&quot;"
	}
	return ""
}

// put puts r onto the current cursor's position, then advances the cursor.
func (v *VT100) put(r rune) {
	v.scrollIfNeeded()
	v.Content[v.Cursor.Y][v.Cursor.X] = r
	v.Format[v.Cursor.Y][v.Cursor.X] = v.Cursor.F
	v.advance()
}

// advance advances the cursor, wrapping to the next line if need be.
func (v *VT100) advance() {
	v.Cursor.X++
	if v.Cursor.X >= v.Width {
		v.Cursor.X = 0
		v.Cursor.Y++
	}
	// if v.Cursor.Y >= v.Height {
	// 	// TODO(jaguilar): if we implement scroll, this should probably scroll.
	// 	// v.Cursor.Y = 0
	// 	v.scroll()
	// }
}

func (v *VT100) scrollIfNeeded() {
	if v.Cursor.X >= v.Width {
		v.Cursor.X = 0
		v.Cursor.Y++
	}
	if v.Cursor.Y >= v.Height {
		first := v.Content[0]
		copy(v.Content, v.Content[1:])
		for i := range first {
			first[i] = ' '
		}
		v.Content[v.Height-1] = first
		v.Cursor.Y = v.Height - 1
	}
}

// home moves the cursor to the coordinates y x. If y x are out of bounds, v.Err
// is set.
func (v *VT100) home(y, x int) {
	v.Cursor.Y, v.Cursor.X = y, x
}

// eraseDirection is the logical direction in which an erase command happens,
// from the cursor. For both erase commands, forward is 0, backward is 1,
// and everything is 2.
type eraseDirection int

const (
	// From the cursor to the end, inclusive.
	eraseForward eraseDirection = iota

	// From the beginning to the cursor, inclusive.
	eraseBack

	// Everything.
	eraseAll
)

// eraseColumns erases columns from the current line.
func (v *VT100) eraseColumns(d eraseDirection) {
	y, x := v.Cursor.Y, v.Cursor.X // Aliases for simplicity.
	switch d {
	case eraseBack:
		v.eraseRegion(y, 0, y, x)
	case eraseForward:
		v.eraseRegion(y, x, y, v.Width-1)
	case eraseAll:
		v.eraseRegion(y, 0, y, v.Width-1)
	}
}

// eraseLines erases lines from the current terminal. Note that
// no matter what is selected, the entire current line is erased.
func (v *VT100) eraseLines(d eraseDirection) {
	y := v.Cursor.Y // Alias for simplicity.
	switch d {
	case eraseBack:
		v.eraseRegion(0, 0, y, v.Width-1)
	case eraseForward:
		v.eraseRegion(y, 0, v.Height-1, v.Width-1)
	case eraseAll:
		v.eraseRegion(0, 0, v.Height-1, v.Width-1)
	}
}

func (v *VT100) eraseRegion(y1, x1, y2, x2 int) {
	// Do not sanitize or bounds-check these coordinates, since they come from the
	// programmer (me). We should panic if any of them are out of bounds.
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}

	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			v.clear(y, x)
		}
	}
}

func (v *VT100) clear(y, x int) {
	if y >= len(v.Content) || x >= len(v.Content[0]) {
		return
	}
	v.Content[y][x] = ' '
	v.Format[y][x] = Format{}
}

func (v *VT100) backspace() {
	v.Cursor.X--
	if v.Cursor.X < 0 {
		if v.Cursor.Y == 0 {
			v.Cursor.X = 0
		} else {
			v.Cursor.Y--
			v.Cursor.X = v.Width - 1
		}
	}
}

func (v *VT100) save() {
	v.savedCursor = v.Cursor
}

func (v *VT100) unsave() {
	v.Cursor = v.savedCursor
}
