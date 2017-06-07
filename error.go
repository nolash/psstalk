package main

import (
	"fmt"
)

const (
	eInit = iota
	ePss
)

var (
	pssTalkErrorString = map[int]string{
		eInit: "Initialize",
		ePss: "Pss backend",
	}
)

type pssTalkError struct {
	code int
	info string
}

func newError(code int, info string) error {
	return &pssTalkError{
		code: code,
		info: info,
	}
}

func (e *pssTalkError) Error() string {
	return fmt.Sprintf("%s: %s", pssTalkErrorString[e.code], e.info)
}


