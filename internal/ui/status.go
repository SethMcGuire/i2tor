package ui

import (
	"fmt"
	"io"
)

func PrintStatus(w io.Writer, message string, fields map[string]string) {
	fmt.Fprintln(w, message)
	for key, value := range fields {
		fmt.Fprintf(w, "  %s: %s\n", key, value)
	}
}
