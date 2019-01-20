package stacktrace

import (
	"errors"
)

/*
RootCause unwraps the original error that caused the current one.

	_, err := f()
	if perr, ok := Stacktrace.RootCause(err).(*ParsingError); ok {
		showError(perr.Line, perr.Column, perr.Text)
	}
*/
func RootCause(err error) error {
	for {
		st, ok := err.(*Stacktrace)
		if !ok {
			return err
		}
		if st.cause == nil {
			return errors.New(st.message)
		}
		err = st.cause
	}
}
