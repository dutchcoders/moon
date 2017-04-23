package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var formatRe = regexp.MustCompile(`%{([a-z]+)(?::(.*?[^\\]))?}`)

var (
	layouts = map[string]string{
		"time":  "20060102",
		"color": "reset",
	}
	formatters = map[string]func(*value, string) string{
		"time":    timeFormatter,
		"message": messageFormatter,
		"host":    hostFormatter,
		"color":   colorFormatter,
	}
)

func Date(t time.Time) func(v *value) {
	return func(v *value) {
		v.Time = t
	}
}

func Host(s string) func(v *value) {
	return func(v *value) {
		v.Host = s
	}
}

func Message(s string) func(v *value) {
	return func(v *value) {
		v.Message = s
	}
}

type value struct {
	Time time.Time

	Message string
	Host    string
}

type optionFn func(v *value)

func timeFormatter(v *value, layout string) string {
	return v.Time.Format(layout)
}

func hostFormatter(v *value, layout string) string {
	return v.Host
}

func messageFormatter(v *value, layout string) string {
	return v.Message
}

func ColorSeq(color color) string {
	return fmt.Sprintf("\033[%dm", int(color))
}

func ColorSeqBold(color color) string {
	return fmt.Sprintf("\033[%d;1m", int(color))

}

type color int

// Base attributes
const (
	Reset color = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	ColorBlack color = iota + 30
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

var (
	colors = map[string]color{
		"black":   ColorBlack,
		"red":     ColorRed,
		"green":   ColorGreen,
		"yellow":  ColorYellow,
		"blue":    ColorBlue,
		"magenta": ColorMagenta,
		"cyan":    ColorCyan,
		"white":   ColorWhite,

		"hiblack":   color(ColorBlack + 60),
		"hired":     color(ColorRed + 60),
		"higreen":   color(ColorGreen + 60),
		"hiyellow":  color(ColorYellow + 60),
		"hiblue":    color(ColorBlue + 60),
		"himagenta": color(ColorMagenta + 60),
		"hicyan":    color(ColorCyan + 60),
		"hiwhite":   color(ColorWhite + 60),

		"bg-black":   color(ColorBlack + 10),
		"bg-red":     color(ColorRed + 10),
		"bg-green":   color(ColorGreen + 10),
		"bg-yellow":  color(ColorYellow + 10),
		"bg-blue":    color(ColorBlue + 10),
		"bg-magenta": color(ColorMagenta + 10),
		"bg-cyan":    color(ColorCyan + 10),
		"bg-white":   color(ColorWhite + 10),

		"bg-hiblack":   color(ColorBlack + 70),
		"bg-hired":     color(ColorRed + 70),
		"bg-higreen":   color(ColorGreen + 70),
		"bg-hiyellow":  color(ColorYellow + 70),
		"bg-hiblue":    color(ColorBlue + 70),
		"bg-himagenta": color(ColorMagenta + 70),
		"bg-hicyan":    color(ColorCyan + 70),
		"bg-hiwhite":   color(ColorWhite + 70),
	}
)

func colorFormatter(v *value, layout string) string {
	if layout == "reset" {
		return "\033[0m"
	} else {
		args := strings.Split(layout, ";")

		bla := make([]string, len(args))
		for i := range args {
			if v, ok := colors[args[i]]; ok {
				bla[i] = fmt.Sprintf("%d", int(v))
			} else {
				bla[i] = fmt.Sprintf("%s", args[i])
			}
		}

		return fmt.Sprintf("\033[%sm", strings.Join(bla, ";"))
	}
}

func String(format string, options ...optionFn) string {
	v := &value{
		Time: time.Now(),
	}

	for _, option := range options {
		option(v)
	}

	matches := formatRe.FindAllStringSubmatchIndex(format, -1)
	if matches == nil {
		return format
	}

	prev := 0

	str := ""

	for _, m := range matches {
		str += format[prev:m[0]]

		name := format[m[2]:m[3]]

		layout := ""
		if v, ok := layouts[name]; ok {
			layout = v
		}

		if m[4] != -1 {
			layout = format[m[4]:m[5]]
		}

		if formatter, ok := formatters[name]; ok {
			str += formatter(v, layout)
		} else {
			str += "%INVALID"
		}

		prev = m[1]
	}

	str += format[prev:]

	return str
}
