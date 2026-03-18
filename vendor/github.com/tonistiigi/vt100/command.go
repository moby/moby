package vt100

import (
	"errors"
	"expvar"
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"
)

// UnsupportedError indicates that we parsed an operation that this
// terminal does not implement. Such errors indicate that the client
// program asked us to perform an action that we don't know how to.
// It MAY be safe to continue trying to do additional operations.
// This is a distinct category of errors from things we do know how
// to do, but are badly encoded, or errors from the underlying io.RuneScanner
// that we're reading commands from.
type UnsupportedError struct {
	error
}

var (
	supportErrors = expvar.NewMap("vt100-unsupported-operations")
)

func supportError(e error) error {
	supportErrors.Add(e.Error(), 1)
	return UnsupportedError{e}
}

// Command is a type of object that the terminal can process to perform
// an update.
type Command interface {
	display(v *VT100) error
}

// runeCommand is a simple command that just writes a rune
// to the current cell and advances the cursor.
type runeCommand rune

func (r runeCommand) display(v *VT100) error {
	v.put(rune(r))
	return nil
}

// escapeCommand is a control sequence command. It includes a variety
// of control and escape sequences that move and modify the cursor
// or the terminal.
type escapeCommand struct {
	cmd  rune
	args string
}

func (c escapeCommand) String() string {
	return fmt.Sprintf("[%q %U](%v)", c.cmd, c.cmd, c.args)
}

type intHandler func(*VT100, []int) error

var (
	// intHandlers are handlers for which all arguments are numbers.
	// This is most of them -- all the ones that we process. Eventually,
	// we may add handlers that support non-int args. Those handlers
	// will instead receive []string, and they'll have to choose on their
	// own how they might be parsed.
	intHandlers = map[rune]intHandler{
		's': save,
		'7': save,
		'u': unsave,
		'8': unsave,
		'A': relativeMove(-1, 0),
		'B': relativeMove(1, 0),
		'C': relativeMove(0, 1),
		'D': relativeMove(0, -1),
		'K': eraseColumns,
		'J': eraseLines,
		'H': home,
		'f': home,
		'm': updateAttributes,
	}
)

func save(v *VT100, _ []int) error {
	v.save()
	return nil
}

func unsave(v *VT100, _ []int) error {
	v.unsave()
	return nil
}

var (
	codeColors = []color.RGBA{
		Black,
		Red,
		Green,
		Yellow,
		Blue,
		Magenta,
		Cyan,
		White,
		{}, // Not used.
		DefaultColor,
	}
)

// A command to update the attributes of the cursor based on the arg list.
func updateAttributes(v *VT100, args []int) error {
	f := &v.Cursor.F

	var unsupported []int
	for _, x := range args {
		switch x {
		case 0:
			*f = Format{}
		case 1:
			f.Intensity = Bright
		case 2:
			f.Intensity = Dim
		case 22:
			f.Intensity = Normal
		case 4:
			f.Underscore = true
		case 24:
			f.Underscore = false
		case 5, 6:
			f.Blink = true // We don't distinguish between blink speeds.
		case 25:
			f.Blink = false
		case 7:
			f.Inverse = true
		case 27:
			f.Inverse = false
		case 8:
			f.Conceal = true
		case 28:
			f.Conceal = false
		case 30, 31, 32, 33, 34, 35, 36, 37, 39:
			f.Fg = codeColors[x-30]
		case 40, 41, 42, 43, 44, 45, 46, 47, 49:
			f.Bg = codeColors[x-40]
			// 38 and 48 not supported. Maybe someday.
		default:
			unsupported = append(unsupported, x)
		}
	}

	if unsupported != nil {
		return supportError(fmt.Errorf("unknown attributes: %v", unsupported))
	}
	return nil
}

func relativeMove(y, x int) func(*VT100, []int) error {
	return func(v *VT100, args []int) error {
		c := 1
		if len(args) >= 1 {
			c = args[0]
		}
		// home is 1-indexed, because that's what the terminal sends us. We want to
		// reuse its sanitization scheme, so we'll just modify our args by that amount.
		return home(v, []int{v.Cursor.Y + y*c + 1, v.Cursor.X + x*c + 1})
	}
}

func eraseColumns(v *VT100, args []int) error {
	d := eraseForward
	if len(args) > 0 {
		d = eraseDirection(args[0])
	}
	if d > eraseAll {
		return fmt.Errorf("unknown erase direction: %d", d)
	}
	v.eraseColumns(d)
	return nil
}

func eraseLines(v *VT100, args []int) error {
	d := eraseForward
	if len(args) > 0 {
		d = eraseDirection(args[0])
	}
	if d > eraseAll {
		return fmt.Errorf("unknown erase direction: %d", d)
	}
	v.eraseLines(d)
	return nil
}

func sanitize(v *VT100, y, x int) (int, int, error) {
	var err error
	if y < 0 || y >= v.Height || x < 0 || x >= v.Width {
		err = fmt.Errorf("out of bounds (%d, %d)", y, x)
	} else {
		return y, x, nil
	}

	if y < 0 {
		y = 0
	}
	if y >= v.Height {
		y = v.Height - 1
	}
	if x < 0 {
		x = 0
	}
	if x >= v.Width {
		x = v.Width - 1
	}
	return y, x, err
}

func home(v *VT100, args []int) error {
	var y, x int
	if len(args) >= 2 {
		y, x = args[0]-1, args[1]-1 // home args are 1-indexed.
	}
	y, x, err := sanitize(v, y, x) // Clamp y and x to the bounds of the terminal.
	v.home(y, x)                   // Try to do something like what the client asked.
	return err
}

func (c escapeCommand) display(v *VT100) error {
	f, ok := intHandlers[c.cmd]
	if !ok {
		return supportError(c.err(errors.New("unsupported command")))
	}

	args, err := c.argInts()
	if err != nil {
		return c.err(fmt.Errorf("while parsing int args: %v", err))
	}

	return f(v, args)
}

// err enhances e with information about the current escape command
func (c escapeCommand) err(e error) error {
	return fmt.Errorf("%s: %s", c, e)
}

var csArgsRe = regexp.MustCompile("^([^0-9]*)(.*)$")

// argInts parses c.args as a slice of at least arity ints. If the number
// of ; separated arguments is less than arity, the remaining elements of
// the result will be zero. errors only on integer parsing failure.
func (c escapeCommand) argInts() ([]int, error) {
	if len(c.args) == 0 {
		return make([]int, 0), nil
	}
	args := strings.Split(c.args, ";")
	out := make([]int, len(args))
	for i, s := range args {
		x, err := strconv.ParseInt(s, 10, 0)
		if err != nil {
			return nil, err
		}
		out[i] = int(x)
	}
	return out, nil
}

type controlCommand rune

const (
	backspace      controlCommand = '\b'
	_horizontalTab                = '\t'
	linefeed                      = '\n'
	_verticalTab                  = '\v'
	_formfeed                     = '\f'
	carriageReturn                = '\r'
)

func (c controlCommand) display(v *VT100) error {
	switch c {
	case backspace:
		v.backspace()
	case linefeed:
		v.Cursor.Y++
		v.Cursor.X = 0
	case carriageReturn:
		v.Cursor.X = 0
	}
	return nil
}
