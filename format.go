// Copyright 2016 Palantir Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stacktrace

import (
	"fmt"
	"strings"
)

/*
DefaultFormat defines the behavior of err.Error() when called on a Stacktrace,
as well as the default behavior of the "%v", "%s" and "%q" formatting
specifiers. By default, all of these produce a full Stacktrace including line
number information. To have them produce a condensed single-line output, set
this value to Stacktrace.FormatBrief.

The formatting specifier "%+s" can be used to force a full Stacktrace regardless
of the value of DefaultFormat. Similarly, the formatting specifier "%#s" can be
used to force a brief output.
*/
var DefaultFormat = FormatFull

// Format is the type of the two possible values of Stacktrace.DefaultFormat.
type Format int

const (
	// FormatFull means format as a full Stacktrace including line number information.
	FormatFull Format = iota
	// FormatBrief means Format on a single line without line number information.
	FormatBrief
)

var _ fmt.Formatter = (*Stacktrace)(nil)

func (st *Stacktrace) Format(f fmt.State, c rune) {
	var text string
	if f.Flag('+') && !f.Flag('#') && c == 's' { // "%+s"
		text = formatFull(st)
	} else if f.Flag('#') && !f.Flag('+') && c == 's' { // "%#s"
		text = formatBrief(st)
	} else {
		text = map[Format]func(*Stacktrace) string{
			FormatFull:  formatFull,
			FormatBrief: formatBrief,
		}[DefaultFormat](st)
	}

	formatString := "%"
	// keep the flags recognized by fmt package
	for _, flag := range "-+# 0" {
		if f.Flag(int(flag)) {
			formatString += string(flag)
		}
	}
	if width, has := f.Width(); has {
		formatString += fmt.Sprint(width)
	}
	if precision, has := f.Precision(); has {
		formatString += "."
		formatString += fmt.Sprint(precision)
	}
	formatString += string(c)
	fmt.Fprintf(f, formatString, text)
}

func formatFull(st *Stacktrace) string {
	var str string
	newline := func() {
		if str != "" && !strings.HasSuffix(str, "\n") {
			str += "\n"
		}
	}

	for curr, ok := st, true; ok; curr, ok = curr.cause.(*Stacktrace) {
		str += curr.message

		if curr.file != "" {
			newline()
			if curr.function == "" {
				str += fmt.Sprintf(" --- at %v:%v ---", curr.file, curr.line)
			} else {
				str += fmt.Sprintf(" --- at %v:%v (%v) ---", curr.file, curr.line, curr.function)
			}
		}

		if curr.cause != nil {
			newline()
			if cause, ok := curr.cause.(*Stacktrace); !ok {
				str += "Caused by: "
				str += curr.cause.Error()
			} else if cause.message != "" {
				str += "Caused by: "
			}
		}
	}

	return str
}

func formatBrief(st *Stacktrace) string {
	var str string
	concat := func(msg string) {
		if str != "" && msg != "" {
			str += ": "
		}
		str += msg
	}

	curr := st
	for {
		concat(curr.message)
		if cause, ok := curr.cause.(*Stacktrace); ok {
			curr = cause
		} else {
			break
		}
	}
	if curr.cause != nil {
		concat(curr.cause.Error())
	}
	return str
}
