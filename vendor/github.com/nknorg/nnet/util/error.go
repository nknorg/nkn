package util

import (
	"errors"
	"strings"
)

// Errors is a list of errors
type Errors []error

// NewErrors creates a new Errors
func NewErrors() Errors {
	errs := make(Errors, 0)
	return errs
}

// Merged returns a single error containing the string representation of all
// errors in Errors, or nil if no err in errs
func (errs Errors) Merged() error {
	if errs == nil || len(errs) == 0 {
		return nil
	}

	errStrings := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
	}
	return errors.New(strings.Join(errStrings, "; "))
}
