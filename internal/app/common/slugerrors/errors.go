package slugerrors

import "errors"

type SlugError struct {
	Slug    string
	Message string
	Err     error
}

func NewSlugError(slug, message string) *SlugError {
	return &SlugError{Slug: slug, Message: message}
}

func NewSlugErrorWrap(slug, message string, err error) *SlugError {
	return &SlugError{Slug: slug, Message: message, Err: err}
}

func (e *SlugError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *SlugError) Unwrap() error {
	return e.Err
}

func GetSlugError(err error) (*SlugError, bool) {
	var slugErr *SlugError
	if errors.As(err, &slugErr) {
		return slugErr, true
	}
	return nil, false
}
