package main

import (
	"fmt"
	"io"
)

type ansi struct {
	o io.Writer
}

func NewTerminal(o io.Writer) ansi {
	return ansi{
		o: o,
	}
}

func (a ansi) Reset() ansi {
	fmt.Fprint(a.o, "\x1b[2J\x1b[;H")
	return a
}

func (a ansi) DisableCursor() ansi {
	fmt.Fprint(a.o, "\x1b[?25l")
	return a
}

func (a ansi) EnableCursor() ansi {
	fmt.Fprint(a.o, "\x1b[?25h")
	return a
}
