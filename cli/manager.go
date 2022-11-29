package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/omushpapa/routerman/core"
)

func RunMenuActions(env *core.Env, actions []*Action) (Navigation, error) {
	if QuitProgram(env.Ctx) {
		return BACK, nil
	}

	var (
		options      strings.Builder
		navigation   Navigation
		containsQuit bool = false
	)
	for i, action := range actions {
		id := strconv.Itoa(i + 1)
		if action == ActionQuit {
			containsQuit = true
			id = "Q"
		}
		options.WriteString(
			fmt.Sprintf("%s: %s\n", id, action.Name),
		)
	}
	if !containsQuit {
		options.WriteString("B: Back\n")
		options.WriteString("Q: Quit\n")
	}

	for {
		fmt.Fprintf(env.Out, "\nChoose an action: \n%s\n\nChoice: ", options.String())
		choice, err := GetChoiceInput(env.In, len(actions))
		if err != nil {
			if err == ErrInvalidChoice || err == ErrInvalidInput {
				fmt.Fprintf(env.Out, "%v, try again\n", err)
				continue
			} else {
				return NEXT, err
			}
		}

		if choice == ExitChoice {
			break
		}

		if choice == QuitChoice {
			env.Ctx.Set("quit", 1)
			break
		}

		action := actions[choice]
		if action == ActionQuit {
			env.Ctx.Set("quit", 1)
			break
		}

		if action.Action != nil {
			navigation, err = action.Action(env)
			if err != nil {
				return NEXT, err
			}

			if navigation == BACK {
				break
			}

			if navigation == REPEAT {
				continue
			}
		}

		children := action.GetValidChildren(env.Ctx)
		if len(children) > 0 {
			navigation, err = RunMenuActions(env, children)
			if QuitProgram(env.Ctx) {
				break
			}

			if err != nil {
				return NEXT, err
			}

			if navigation == BACK {
				break
			}
		}
	}
	return NEXT, nil
}

func (action Action) GetValidChildren(ctx core.Context) []*Action {
	actions := make([]*Action, 0)

OUTER:
	for _, action := range action.Children {
		if len(action.RequiresContext) > 0 {
			for _, k := range action.RequiresContext {
				_, exists := ctx[k]
				if !exists {
					continue OUTER
				}
			}
		}
		actions = append(actions, action)
	}
	return actions
}

func QuitProgram(ctx core.Context) bool {
	quit := ctx["quit"]
	return quit > 0
}
