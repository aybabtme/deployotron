package db

import (
	"io"
	"strings"
)

type text interface {
	AsString() string
	AsBytes() []byte
	AsReader() io.ReadSeeker
}

type key text
type value text

type stringTxt string

func (txt stringTxt) AsString() string        { return string(txt) }
func (txt stringTxt) AsBytes() []byte         { return []byte(txt) }
func (txt stringTxt) AsReader() io.ReadSeeker { return strings.NewReader(txt.AsString()) }

type kvStore interface {
	CondPut(key key, new, old value) error
	Get(key key) (value, bool, error)
}
