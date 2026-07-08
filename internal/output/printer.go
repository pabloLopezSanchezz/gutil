package output

import (
	"fmt"
	"io"
)

type Printer struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (p Printer) Info(message string)    { fmt.Fprintln(p.Stdout, message) }
func (p Printer) Success(message string) { fmt.Fprintln(p.Stdout, message) }
func (p Printer) Warning(message string) { fmt.Fprintln(p.Stderr, "Warning:", message) }
func (p Printer) Error(message string)   { fmt.Fprintln(p.Stderr, "Error:", message) }
