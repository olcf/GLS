package columnize

import (
	"os"
	"text/tabwriter"
	"fmt"
	"strings"
)

type Color string

var writer *tabwriter.Writer

const (
	Reset Color           = "\x1b[000000m"
	Green                 = "\x1b[000032m"
	Yellow                = "\x1b[000033m"
	Red                   = "\x1b[000031m"
	Blue                  = "\x1b[000034m"
	LightBlue             = "\x1b[000036m"
	White                 = "\x1b[000037m"
	BlinkingRedBackground = "\x1b[0041;5m"
)

func (c *Color) String() string {
	return string(*c)
}

func Colorize(c Color, s string) string {
	return string(c) + s + string(Reset)
}

func ColumnizeRow(c Color, idx int, val []string) []string {
	var row []string
	for i,v := range val {
		if i == idx {
			row = append(row, Colorize(c, v))
		} else {
			row = append(row, v)
		}
	}
	return row
}

func ColumnizeRowMultiColor(data map[string]Color) []string {
	var row []string
	for str, color := range data {
		row = append(row, Colorize(color, str))
	}
	return row
}

func PrintLine(line []string) {
	fmt.Fprintln(writer, strings.Join(line, "\t"))
}

func New() {
	writer = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

func NewAlignRight() {
	writer = tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', tabwriter.AlignRight|tabwriter.DiscardEmptyColumns)
}

func Flush() {
	writer.Flush()
}

func getWriter() *tabwriter.Writer {
	return writer
}
