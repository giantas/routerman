package cli

import (
	"errors"

	"github.com/omushpapa/routerman/core"
)

var (
	ErrInvalidChoice = errors.New("invalid choice")
	ErrInvalidInput  = errors.New("invalid input")
	ExitChoice       = 99
	QuitChoice       = 999
)

type Navigation int

const (
	NEXT Navigation = iota
	BACK
	REPEAT
)

type ActionFunc func(env *core.Env) (Navigation, error)

type Action struct {
	Name            string
	Children        []*Action
	RequiresContext []string
	Action          ActionFunc
}
