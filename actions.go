package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/giantas/routerman/storage"
)

var (
	ErrInvalidChoice = errors.New("invalid choice")
	ErrInvalidInput  = errors.New("invalid input")
	ExitChoice       = 99
)

type Context map[string]int

func (ctx Context) Set(key string, value int) {
	ctx[key] = value
}

type ActionFunc func(in io.Reader, db *storage.Store, ctx Context) (bool, error)

type Action struct {
	Name            string
	Action          ActionFunc
	Children        []*Action
	RequiresContext []string
}

func (action Action) GetValidChildren(ctx Context) []*Action {
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

var RootActionManageUsers = &Action{
	Name: "Manage users",
	Children: []*Action{
		ActionRegisterUser,
		ActionListUsers,
	},
}

var ActionRegisterUser = &Action{
	Name: "Register a user",
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		fmt.Printf("Name: ")
		name, err := GetInput(in)
		if err != nil {
			return false, err
		}
		if name == "" {
			return false, ErrInvalidInput
		}
		user := &storage.User{
			Name: name,
		}
		err = db.UserStore.Create(user)
		if err != nil {
			return false, err
		}
		fmt.Printf("user %+v created", user)
		return false, nil
	},
}

var ActionListUsers = &Action{
	Name: "List users",
	Children: []*Action{
		ActionListUserSlots,
		ActionDeregisterUser,
		ActionRegisterDevice,
		ActionListDevices,
	},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			users      []storage.User
			err        error
		)

		for {
			if showList {
				users, err = db.UserStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return false, err
				}
				if len(users) == 0 {
					fmt.Println("no users found")
					return false, nil
				}
				for i, user := range users {
					fmt.Printf("%d. %s\n", i+1, user.Name)
				}
			}

			fmt.Printf("\nSelect user by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return false, err
			}

			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return false, nil
			default:
				position, err := GetChoice(choice, len(users))
				if err == ErrInvalidChoice {
					fmt.Println("invalid choice. try again")
					showList = false
					continue
				}

				user := users[position]
				userId := user.Id

				fmt.Printf("Selected user '%s'\n", user.Name)

				_, err = db.UserStore.Read(userId)
				if err != nil {
					return false, err
				}

				ctx.Set("userId", userId)
				return false, err
			}
			showList = true
		}
	},
}

var ActionListUserSlots = &Action{
	Name: "List user slots",
	Children: []*Action{
		ActionDeleteSlot,
	},
	RequiresContext: []string{"userId"},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		userId, exists := ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}

		var (
			err        error
			slots      []storage.BandwidthSlot
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
		)
		fmt.Println("\nListing user slots ")

		for {
			if showList {
				slots, err = db.BandwidthSlotStore.ReadManyByUserId(userId, pageSize, pageNumber)
				if err != nil {
					return false, err
				}

				if len(slots) == 0 {
					fmt.Println("no slots found")
					return false, nil
				}

				for i, slot := range slots {
					fmt.Printf("%d. %d:%d\n", i+1, slot.Id, slot.RemoteId)
				}
			}

			fmt.Printf("\nSelect slot by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return false, err
			}

			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return false, nil
			default:
				position, err := GetChoice(choice, len(slots))
				if err == ErrInvalidChoice {
					fmt.Println("invalid choice. try again")
					showList = false
					continue
				}

				slotId := slots[position].Id
				_, err = db.BandwidthSlotStore.Read(slotId)
				if err != nil {
					return false, err
				}

				ctx.Set("slotId", slotId)
				return false, err
			}
			showList = true
		}
	},
}

var ActionDeregisterUser = &Action{
	Name:            "Deregister user",
	RequiresContext: []string{"userId"},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		userId, exists := ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}
		actions := []func(userId int) error{
			db.BandwidthSlotStore.DeleteByUserId,
			db.DeviceStore.DeleteByUserId,
			db.UserStore.Delete,
		}
		for _, action := range actions {
			err := action(userId)
			if err != nil {
				return false, err
			}
		}
		fmt.Println("user deleted")
		delete(ctx, "userId")
		return true, nil
	},
}

var ActionDeleteSlot = &Action{
	Name: "Delete slot",
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		slotId, exists := ctx["slotId"]
		if !exists {
			return false, fmt.Errorf("slot id not provided")
		}
		err := db.BandwidthSlotStore.Delete(slotId)
		if err != nil {
			return false, err
		}
		return true, nil
	},
	RequiresContext: []string{"slotId"},
}

var RootActionManageDevices = &Action{
	Name: "Manage devices",
	Children: []*Action{
		ActionListDevices,
	},
}

var ActionListDevices = &Action{
	Name: "List devices",
	Children: []*Action{
		ActionDeregisterDevice,
	},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			devices    []storage.Device
			err        error
		)
		userId, userIdProvided := ctx["userId"]

		for {
			if showList {
				if userIdProvided && userId != 0 {
					devices, err = db.DeviceStore.ReadManyByUserId(userId, pageSize, pageNumber)
				} else {
					devices, err = db.DeviceStore.ReadMany(pageSize, pageNumber)
				}

				if err != nil {
					return false, err
				}
				if len(devices) == 0 {
					fmt.Println("no devices found")
					return false, nil
				}
				for i, device := range devices {
					fmt.Printf("%d. %s(%s)\n", i+1, device.Alias, device.Mac)
				}
			}
			fmt.Printf("\nSelect device by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return false, err
			}
			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return false, nil
			default:
				num, err := GetChoice(choice, len(devices))
				if err == ErrInvalidChoice {
					fmt.Println("invalid choice. try again")
					showList = false
					continue
				}
				deviceId := devices[num].Id
				ctx.Set("deviceId", deviceId)
				return false, nil
			}
			showList = true
		}
	},
}

var ActionRegisterDevice = &Action{
	Name:            "Register a device",
	RequiresContext: []string{"userId"},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		userId, exists := ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}

		for {
			fmt.Printf("Enter mac address: ")
			text, err := GetInput(in)
			if err != nil {
				return false, err
			}
			if !IsValidMacAddress(text) {
				fmt.Println("Invalid mac address. Try again")
				continue
			}
			mac := text

			fmt.Printf("Enter alias: ")
			text, err = GetInput(in)
			if err != nil {
				return false, err
			}
			alias := text
			device := storage.Device{
				UserId: userId,
				Mac:    mac,
				Alias:  alias,
			}
			err = db.DeviceStore.Create(&device)
			if err != nil {
				return false, err
			}
			fmt.Printf("Device added successfully %+v\n", device)
			break
		}
		return false, nil
	},
}

var ActionDeregisterDevice = &Action{
	Name:            "Deregister device",
	RequiresContext: []string{"deviceId"},
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		deviceId, exists := ctx["deviceId"]
		if !exists {
			return false, fmt.Errorf("device id not provided")
		}
		err := db.DeviceStore.Delete(deviceId)
		if err != nil {
			return false, err
		}

		fmt.Println("Device deregistered")
		delete(ctx, "deviceId")
		return true, nil
	},
}

var Quit = &Action{
	Name: "Quit",
	Action: func(in io.Reader, db *storage.Store, ctx Context) (bool, error) {
		return false, nil
	},
}

func RunMenuActions(in io.Reader, store *storage.Store, actions []*Action, ctx Context) (bool, error) {
	var (
		options      strings.Builder
		goBack       bool
		containsQuit bool = false
	)
	for i, action := range actions {
		id := strconv.Itoa(i + 1)
		if action == Quit {
			containsQuit = true
			id = "00"
		}
		options.WriteString(
			fmt.Sprintf("%s: %s\n", id, action.Name),
		)
	}
	if !containsQuit {
		options.WriteString("00: Back\n")
	}

	for {
		fmt.Printf("Choose an action: \n%s\n\nChoice: ", options.String())
		choice, err := GetChoiceInput(in, len(actions))
		if err != nil {
			if err == ErrInvalidChoice || err == ErrInvalidInput {
				fmt.Printf("%v, try again\n", err)
				continue
			} else {
				return false, err
			}
		}

		if choice == ExitChoice {
			break
		}

		action := actions[choice]
		if action == Quit {
			break
		}

		if action.Action != nil {
			goBack, err = action.Action(in, store, ctx)
			if err != nil {
				return false, err
			}
		}

		children := action.GetValidChildren(ctx)
		if len(children) > 0 {
			goBack, err = RunMenuActions(in, store, children, ctx)
			if err != nil {
				return false, err
			}
		}

		if goBack {
			break
		}
	}
	return false, nil
}

func GetChoiceInput(in io.Reader, max int) (int, error) {
	input, err := GetInput(in)
	if err != nil {
		return 0, err
	}
	if input == "00" {
		return ExitChoice, err
	}
	return GetChoice(input, max)
}

func GetChoice(value string, max int) (int, error) {
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, ErrInvalidChoice
	}
	if num < 1 || num > max {
		return 0, ErrInvalidChoice
	}
	return num - 1, err
}
