// Copyright 2016 Palantir Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this File except in compliance with the License.
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
	"math"
	"runtime"
	"strings"

	"github.com/palantir/stacktrace/cleanpath"
)

/*
CleanPath Function is applied to File paths before adding them to a Stacktrace.
By default, it makes the path relative to the $GOPATH environment variable.

To remove some additional prefix like "github.com" from File paths in
stacktraces, use something like:

	Stacktrace.CleanPath = func(path string) string {
		path = cleanpath.RemoveGoPath(path)
		path = strings.TrimPrefix(path, "github.com/")
		return path
	}
*/
var CleanPath = cleanpath.RemoveGoPath

/*
NewError is a drop-in replacement for fmt.Errorf that includes Line number
information. The canonical call looks like this:

	if !IsOkay(arg) {
		return Stacktrace.NewError("Expected %v to be okay", arg)
	}
*/
func NewError(msg string, vals ...interface{}) error {
	return create(nil, NoCode, msg, vals...)
}

/*
Propagate wraps an error to include Line number information. The msg and vals
arguments work like the ones for fmt.Errorf.

The Message passed to Propagate should describe the action that failed,
resulting in the Cause. The canonical call looks like this:

	result, err := process(arg)
	if err != nil {
		return nil, Stacktrace.Propagate(err, "Failed to process %v", arg)
	}

To write the Message, ask yourself "what does this call do?" What does
process(arg) do? It processes ${arg}, so the Message is that we failed to
process ${arg}.

Pay attention that the Message is not redundant with the one in err. If it is
not possible to add any useful contextual information beyond what is already
included in an error, msg can be an empty string:

	func Something() error {
		mutex.Lock()
		defer mutex.Unlock()

		err := reallySomething()
		return Stacktrace.Propagate(err, "")
	}

If Cause is nil, Propagate returns nil. This allows elision of some "if err !=
nil" checks.
*/
func Propagate(cause error, msg string, vals ...interface{}) error {
	if cause == nil {
		// Allow calling Propagate without checking whether there is error
		return nil
	}
	return create(cause, NoCode, msg, vals...)
}

/*
ErrorCode is a Code that can be attached to an error as it is passed/propagated
up the stack.

There is no predefined set of error codes. You define the ones relevant to your
application:

	const (
		EcodeManifestNotFound = Stacktrace.ErrorCode(iota)
		EcodeBadInput
		EcodeTimeout
	)

The one predefined error Code is NoCode, which has a value of math.MaxUint16.
Avoid using that value as an error Code.

An ordinary Stacktrace.Propagate call preserves the error Code of an error.
*/
type ErrorCode uint16

/*
NoCode is the error Code of errors with no Code explicitly attached.
*/
const NoCode ErrorCode = math.MaxUint16

/*
NewErrorWithCode is similar to NewError but also attaches an error Code.
*/
func NewErrorWithCode(code ErrorCode, msg string, vals ...interface{}) error {
	return create(nil, code, msg, vals...)
}

/*
PropagateWithCode is similar to Propagate but also attaches an error Code.

	_, err := os.Stat(manifestPath)
	if os.IsNotExist(err) {
		return Stacktrace.PropagateWithCode(err, EcodeManifestNotFound, "")
	}
*/
func PropagateWithCode(cause error, code ErrorCode, msg string, vals ...interface{}) error {
	if cause == nil {
		// Allow calling PropagateWithCode without checking whether there is error
		return nil
	}
	return create(cause, code, msg, vals...)
}

/*
NewMessageWithCode returns an error that prints just like fmt.Errorf with no
Line number, but including a Code. The error Code mechanism can be useful by
itself even where stack traces with Line numbers are not warranted.

	ttl := req.URL.Query().Get("ttl")
	if ttl == "" {
		return 0, Stacktrace.NewMessageWithCode(EcodeBadInput, "Missing ttl query parameter")
	}
*/
func NewMessageWithCode(code ErrorCode, msg string, vals ...interface{}) error {
	return &Stacktrace{
		Message: fmt.Sprintf(msg, vals...),
		Code:    code,
	}
}

/*
GetCode extracts the error Code from an error.

	for i := 0; i < attempts; i++ {
		err := Do()
		if Stacktrace.GetCode(err) != EcodeTimeout {
			return err
		}
		// try a few more times
	}
	return Stacktrace.NewError("timed out after %d attempts", attempts)

GetCode returns the special value Stacktrace.NoCode if err is nil or if there is
no error Code attached to err.
*/
func GetCode(err error) ErrorCode {
	if err, ok := err.(*Stacktrace); ok {
		return err.Code
	}
	return NoCode
}

func GetCause(err error) error {
	if err, ok := err.(*Stacktrace); ok {
		return err.Cause
	}
	return err
}

func GetMessage(err error) error {
	if err, ok := err.(*Stacktrace); ok {
		return fmt.Errorf(err.Message)
	}
	return err
}

type Stacktrace struct {
	Message  string
	Cause    error
	Code     ErrorCode
	File     string
	Function string
	Line     int
}

func create(cause error, code ErrorCode, msg string, vals ...interface{}) error {
	// If no error Code specified, inherit error Code from the Cause.
	if code == NoCode {
		code = GetCode(cause)
	}

	err := &Stacktrace{
		Message: fmt.Sprintf(msg, vals...),
		Cause:   cause,
		Code:    code,
	}

	// Caller of create is NewError or Propagate, so user's Code is 2 up.
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		return err
	}
	if CleanPath != nil {
		file = CleanPath(file)
	}
	err.File, err.Line = file, line

	f := runtime.FuncForPC(pc)
	if f == nil {
		return err
	}
	err.Function = shortFuncName(f)

	return err
}

/* "FuncName" or "Receiver.MethodName" */
func shortFuncName(f *runtime.Func) string {
	// f.Name() is like one of these:
	// - "github.com/palantir/shield/package.FuncName"
	// - "github.com/palantir/shield/package.Receiver.MethodName"
	// - "github.com/palantir/shield/package.(*PtrReceiver).MethodName"
	longName := f.Name()

	withoutPath := longName[strings.LastIndex(longName, "/")+1:]
	withoutPackage := withoutPath[strings.Index(withoutPath, ".")+1:]

	shortName := withoutPackage
	shortName = strings.Replace(shortName, "(", "", 1)
	shortName = strings.Replace(shortName, "*", "", 1)
	shortName = strings.Replace(shortName, ")", "", 1)

	return shortName
}

func (st *Stacktrace) Error() string {
	return fmt.Sprint(st)
}

// ExitCode returns the exit Code associated with the Stacktrace error based on its error Code. If the error Code is
// NoCode, return 1 (default); otherwise, returns the value of the error Code.
func (st *Stacktrace) ExitCode() int {
	if st.Code == NoCode {
		return 1
	}
	return int(st.Code)
}
