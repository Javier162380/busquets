package domain

import "errors"

var ErrNoContentChanges = errors.New("no content changes found between versions")
