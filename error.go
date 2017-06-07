package main

const (
	pssTalkErrorInit = iota
)

var (
	pssTalkErrorString = map[int]string{
		pssTalkErrorInit: "Initialize",
	}
)

type pssTalkError struct {
	code int
}

func newPssTalkError(code int) error {
	return &pssTalkError{
		code: code,
	}
}

func (e *pssTalkError) Error() string {
	return pssTalkErrorString[e.code]
}


