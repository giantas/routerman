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

type ActionFunc func(in io.Reader, db *storage.Store) error

type Action struct {
	Name     string
	Action   ActionFunc
	Children []*Action
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
	Action: func(in io.Reader, db *storage.Store) error {
		fmt.Printf("Name: ")
		name, err := GetInput(in)
		if err != nil {
			return err
		}
		if name == "" {
			return ErrInvalidInput
		}
		user := &storage.User{
			Name: name,
		}
		err = db.UserStore.Create(user)
		if err != nil {
			return err
		}
		fmt.Printf("user %+v created", user)
		return nil
	},
}

var ActionListUsers = &Action{
	Name: "List users",
	Action: func(in io.Reader, db *storage.Store) error {
		pageNumber := 1
		pageSize := 5
		showList := true

		for {
			if showList {
				users, err := db.UserStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return err
				}
				if len(users) == 0 {
					fmt.Println("no users found")
					return nil
				}
				for i, user := range users {
					fmt.Printf("%d. %s\n", i+1, user.Name)
				}
			}
			fmt.Printf("Scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return err
			}
			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return nil
			default:
				fmt.Println("invalid choice. try again")
				showList = false
				continue
			}
			showList = true
		}
	},
}

var RootActionManageDevices = &Action{
	Name: "Manage devices",
	Children: []*Action{
		ActionListDevices,
		ActionRegisterDevice,
		ActionDeregisterDevice,
	},
}

var ActionListDevices = &Action{
	Name: "List devices",
	Action: func(in io.Reader, db *storage.Store) error {
		pageNumber := 1
		pageSize := 5
		showList := true

		for {
			if showList {
				devices, err := db.DeviceStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return err
				}
				if len(devices) == 0 {
					fmt.Println("no devices found")
					return nil
				}
				for i, device := range devices {
					fmt.Printf("%d. %s(%s)\n", i+1, device.Alias, device.Mac)
				}
			}
			fmt.Printf("Scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return err
			}
			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return nil
			default:
				fmt.Println("invalid choice. try again")
				showList = false
				continue
			}
			showList = true
		}
	},
}

var ActionRegisterDevice = &Action{
	Name: "Register a device",
	Action: func(in io.Reader, db *storage.Store) error {
		var (
			err        error
			users      []storage.User
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
		)
		fmt.Println("\nListing users ")

		for {
			if showList {
				users, err = db.UserStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return err
				}

				if len(users) == 0 {
					fmt.Println("no users found")
					return nil
				}

				for i, user := range users {
					fmt.Printf("%d. %s\n", i+1, user.Name)
				}
			}

			fmt.Printf("\nSelect user by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return err
			}

			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return nil
			default:
				position, err := strconv.Atoi(choice)
				if err != nil || position > len(users) || position < 1 {
					fmt.Printf("invalid choice. try again")
					showList = false
					continue
				}

				userId := users[position-1].Id
				_, err = db.UserStore.Read(userId)
				if err != nil {
					return err
				}

				return AddNewDevice(in, db, userId)
			}
			showList = true
		}
	},
}

var ActionDeregisterDevice = &Action{
	Name: "Deregister a device",
	Action: func(in io.Reader, db *storage.Store) error {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			devices    []storage.Device
			err        error
		)

		for {
			if showList {
				devices, err = db.DeviceStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return err
				}
				if len(devices) == 0 {
					fmt.Println("no devices found")
					return nil
				}
				for i, device := range devices {
					fmt.Printf("%d. %s(%s)\n", i+1, device.Alias, device.Mac)
				}
			}
			fmt.Printf("\nSelect user by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(in)
			if err != nil {
				return err
			}
			switch choice {
			case "n":
				pageNumber += 1
			case "p":
				pageNumber -= 1
			case "q":
				return nil
			default:
				num, err := strconv.Atoi(choice)
				if err != nil || num < 1 || num >= len(devices) {
					fmt.Println("invalid choice. try again")
					showList = false
					continue
				}

				deviceId := devices[num-1].Id

				err = db.DeviceStore.Delete(deviceId)
				if err != nil {
					return err
				}

				fmt.Println("Device deregistered")
				return nil
			}
			showList = true
		}
	},
}

var Exit = &Action{
	Name: "Exit",
	Action: func(in io.Reader, db *storage.Store) error {
		return nil
	},
}

func AddNewDevice(in io.Reader, db *storage.Store, userId int) error {
	for {
		fmt.Printf("Enter mac address: ")
		text, err := GetInput(in)
		if err != nil {
			return err
		}
		if !IsValidMacAddress(text) {
			fmt.Println("Invalid mac address. Try again")
			continue
		}
		mac := text

		fmt.Printf("Enter alias: ")
		text, err = GetInput(in)
		if err != nil {
			return err
		}
		alias := text
		device := storage.Device{
			UserId: userId,
			Mac:    mac,
			Alias:  alias,
		}
		err = db.DeviceStore.Create(&device)
		if err != nil {
			return err
		}
		fmt.Printf("Device added successfully %+v\n", device)
		break
	}
	return nil
}

func RunMenuActions(in io.Reader, store *storage.Store, actions []*Action) error {
	var options strings.Builder
	for i, action := range actions {
		options.WriteString(
			fmt.Sprintf("%d: %s\n", i+1, action.Name),
		)
	}
	options.WriteString("00: Exit")

	for {
		fmt.Printf("\nChoose an action: \n%s\n\nChoice: ", options.String())
		choice, err := GetChoice(in, len(actions))
		if err != nil {
			if err == ErrInvalidChoice || err == ErrInvalidInput {
				fmt.Printf("%v, try again\n", err)
				continue
			} else {
				return err
			}
		}

		if choice == ExitChoice {
			break
		}

		action := actions[choice]
		if action == Exit {
			break
		}

		if len(action.Children) > 0 {
			err = RunMenuActions(in, store, action.Children)
		} else {
			err = action.Action(in, store)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func GetChoice(in io.Reader, max int) (int, error) {
	input, err := GetInput(in)
	if err != nil {
		return 0, err
	}
	if input == "00" {
		return ExitChoice, err
	}
	num, err := strconv.Atoi(input)
	if err != nil {
		return 0, ErrInvalidChoice
	}
	if num < 1 || num > max {
		return 0, ErrInvalidChoice
	}
	return num - 1, err
}
