package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/giantas/routerman/storage"
	"github.com/giantas/tplinkapi"
)

var (
	ErrInvalidChoice = errors.New("invalid choice")
	ErrInvalidInput  = errors.New("invalid input")
	ExitChoice       = 99
	QuitChoice       = 999
)

type Context map[string]int

func (ctx Context) Set(key string, value int) {
	ctx[key] = value
}

type ActionFunc func(env *Env) (bool, error)

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
	Action: func(env *Env) (bool, error) {
		fmt.Printf("Name: ")
		name, err := GetInput(env.in)
		if err != nil {
			return false, err
		}
		if name == "" {
			return false, ErrInvalidInput
		}
		user := &storage.User{
			Name: name,
		}
		err = env.db.UserStore.Create(user)
		if err != nil {
			return false, err
		}
		fmt.Printf("user %+v created\n", user)
		return false, nil
	},
}

var ActionListUsers = &Action{
	Name: "List users",
	Children: []*Action{
		ActionListUserBandwidthSlots,
		ActionDeregisterUser,
		ActionListDevices,
	},
	Action: func(env *Env) (bool, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			users      []storage.User
			err        error
		)

		for {
			if showList {
				users, err = env.db.UserStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return false, err
				}

				if len(users) == 0 {
					if pageNumber == 1 {
						fmt.Println("no users found")
						return false, nil
					} else {
						fmt.Println("no more users found")
					}
				}

				for i, user := range users {
					fmt.Printf("%d. %s\n", i+1, user.Name)
				}
			} else {
				fmt.Println("no more users found")
			}

			fmt.Printf("\nSelect user by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(env.in)
			if err != nil {
				return false, err
			}

			switch choice {
			case "n":
				if len(users) == pageSize {
					pageNumber += 1
					showList = true
				} else {
					showList = false
				}
			case "p":
				if pageNumber > 1 {
					pageNumber -= 1
					showList = true
				} else {
					showList = false
				}
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

				_, err = env.db.UserStore.Read(userId)
				if err != nil {
					return false, err
				}

				env.ctx.Set("userId", userId)
				return false, err
			}
		}
	},
}

var ActionListUserBandwidthSlots = &Action{
	Name: "List user bandwidth slots",
	Children: []*Action{
		ActionRegisterDevice,
		ActionAssignSlot,
		ActionDeleteSlot,
	},
	RequiresContext: []string{"userId"},
	Action: func(env *Env) (bool, error) {
		userId, exists := env.ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}

		var (
			err        error
			slots      []storage.BandwidthSlot
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			choice     string
		)
		fmt.Println("\nListing user slots ")

		for {
			if showList {
				slots, err = env.db.BandwidthSlotStore.ReadManyByUserId(userId, pageSize, pageNumber)
				if err != nil {
					return false, err
				}

				ids := make([]int, 0)
				for _, slot := range slots {
					ids = append(ids, slot.RemoteId)
				}

				entries, err := env.router.GetBwControlEntriesByList(ids)
				if err != nil {
					return false, err
				}

				if len(slots) == 0 {
					fmt.Println("no slots found")
					return false, nil
				}

				for i, entry := range entries {
					fmt.Printf(
						"%d. %s - %s Up:%d/%d Down:%d/%d [%v]\n",
						i+1, entry.StartIp, entry.EndIp, entry.UpMin, entry.UpMax, entry.DownMin, entry.DownMax, entry.Enabled,
					)
				}
			} else {
				fmt.Println("no more slots found")
			}

			fmt.Printf("\nSelect slot by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err = GetInput(env.in)
			if err != nil {
				return false, err
			}

			switch choice {
			case "n":
				if len(slots) == pageSize {
					pageNumber += 1
					showList = true
				} else {
					showList = false
				}
			case "p":
				if pageNumber > 1 {
					pageNumber -= 1
					showList = true
				} else {
					showList = false
				}
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
				_, err = env.db.BandwidthSlotStore.Read(slotId)
				if err != nil {
					return false, err
				}

				env.ctx.Set("slotId", slotId)
				return false, err
			}
		}
	},
}

var ActionAssignSlot = &Action{
	Name:            "Assign bandwidth slot",
	RequiresContext: []string{"userId"},
	Action: func(env *Env) (bool, error) {
		userId, exists := env.ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}

		var (
			err        error
			slots      []BwSlot
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			choice     string
		)

		for {
			if showList {
				slots, err = env.router.GetAvailableBandwidthSlots()
				if err != nil {
					return false, err
				}

				if len(slots) == 0 {
					fmt.Println("no slots found")
					goBack := false
					if pageNumber == 1 {
						goBack = true
					}
					return goBack, nil
				}

				for i, slot := range slots {
					cap, err := slot.GetCapacity()
					if err != nil {
						return false, err
					}
					fmt.Printf("%d: %s - %s [%d]\n", i+1, slot.MinAddress, slot.MaxAddress, cap)
				}
			} else {
				fmt.Println("no more slots found")
			}

			fmt.Printf("\nSelect slot by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err = GetInput(env.in)
			if err != nil {
				return false, err
			}

			switch choice {
			case "n":
				if len(slots) == pageSize {
					pageNumber += 1
					showList = true
				} else {
					showList = false
				}
			case "p":
				if pageNumber > 1 {
					pageNumber -= 1
					showList = true
				} else {
					showList = false
				}
			case "q":
				return false, nil
			default:
				position, err := GetChoice(choice, len(slots))
				if err == ErrInvalidChoice {
					return false, fmt.Errorf("invalid choice")
				}
				slot := slots[position]
				capacity, _ := slot.GetCapacity()
				fmt.Printf("Enter number of devices [Default %d]: ", capacity)
				num, err := GetIntInput(env.in, capacity)
				if err != nil {
					return false, err
				}
				if num > capacity || num < 1 {
					return false, fmt.Errorf("invalid number")
				}

				endIP, err := slot.GetMaxIP(num)
				if err != nil {
					return false, err
				}

				maxDown := 1000
				fmt.Printf("Enter max download speed (kbps) [Default %d]: ", maxDown)
				maxDown, err = GetIntInput(env.in, maxDown)
				if err != nil {
					return false, err
				}

				maxUp := 1000
				fmt.Printf("Enter max upload speed (kbps) [Default %d]: ", maxUp)
				maxUp, err = GetIntInput(env.in, maxUp)
				if err != nil {
					return false, err
				}

				entry := tplinkapi.BandwidthControlEntry{
					Enabled: true,
					StartIp: slot.MinAddress,
					EndIp:   endIP,
					UpMin:   50,
					UpMax:   maxUp,
					DownMin: 50,
					DownMax: maxDown,
				}
				id, err := env.router.router.AddBwControlEntry(entry)
				if err != nil {
					return false, err
				}
				storageSlot := storage.BandwidthSlot{
					UserId:   userId,
					RemoteId: id,
				}
				err = env.db.BandwidthSlotStore.Create(&storageSlot)
				if err != nil {
					return false, err
				}
				fmt.Println("Entry created successfully")
				return false, err
			}
		}

	},
}

var ActionDeregisterUser = &Action{
	Name:            "Deregister user",
	RequiresContext: []string{"userId"},
	Action: func(env *Env) (bool, error) {
		userId, exists := env.ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}
		actions := []func(userId int) error{
			env.db.BandwidthSlotStore.DeleteByUserId,
			env.db.DeviceStore.DeleteByUserId,
			env.db.UserStore.Delete,
		}
		for _, action := range actions {
			err := action(userId)
			if err != nil {
				return false, err
			}
		}
		fmt.Println("user deleted")
		delete(env.ctx, "userId")
		return true, nil
	},
}

var ActionDeleteSlot = &Action{
	Name: "Delete slot",
	Action: func(env *Env) (bool, error) {
		slotId, exists := env.ctx["slotId"]
		if !exists {
			return false, fmt.Errorf("slot id not provided")
		}
		slot, err := env.db.BandwidthSlotStore.Read(slotId)
		if err != nil {
			return false, err
		}
		err = env.router.router.DeleteBwControlEntry(slot.RemoteId)
		if err != nil {
			return false, err
		}
		err = env.db.BandwidthSlotStore.Delete(slotId)
		if err != nil {
			return false, err
		}
		fmt.Printf("slot deleted successfully")
		return true, nil
	},
	RequiresContext: []string{"slotId"},
}

var ActionListAvailableSlots = &Action{
	Name: "List available bandwidth slots",
	Action: func(env *Env) (bool, error) {
		slots, err := env.router.GetAvailableBandwidthSlots()
		if err != nil {
			return false, err
		}
		for x, slot := range slots {
			cap, err := slot.GetCapacity()
			if err != nil {
				return false, err
			}
			fmt.Printf("%d: %s - %s [%d]\n", x, slot.MinAddress, slot.MaxAddress, cap)
		}
		return false, nil
	},
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
	Action: func(env *Env) (bool, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			devices    []storage.Device
			err        error
		)
		userId, userIdProvided := env.ctx["userId"]

		for {
			if showList {
				if userIdProvided && userId != 0 {
					devices, err = env.db.DeviceStore.ReadManyByUserId(userId, pageSize, pageNumber)
				} else {
					devices, err = env.db.DeviceStore.ReadMany(pageSize, pageNumber)
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
			} else {
				fmt.Println("no more users found")
			}

			fmt.Printf("\nSelect device by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(env.in)
			if err != nil {
				return false, err
			}
			switch choice {
			case "n":
				if len(devices) == pageSize {
					pageNumber += 1
					showList = true
				} else {
					showList = false
				}
			case "p":
				if pageNumber > 1 {
					pageNumber -= 1
					showList = true
				} else {
					showList = false
				}
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
				env.ctx.Set("deviceId", deviceId)
				return false, nil
			}
		}
	},
}

var ActionRegisterDevice = &Action{
	Name:            "Register a device",
	RequiresContext: []string{"userId", "slotId"},
	Action: func(env *Env) (bool, error) {
		userId, exists := env.ctx["userId"]
		if !exists {
			return false, fmt.Errorf("user id not provided")
		}

		slotId, exists := env.ctx["slotId"]
		if !exists {
			return false, fmt.Errorf("slot id not provided")
		}

		slot, err := env.db.BandwidthSlotStore.Read(slotId)
		if err != nil {
			return false, err
		}

		ipAddress, err := env.router.GetUnusedIPAddress(slot.RemoteId)
		if err != nil {
			return false, err
		}

		_, err = env.db.UserStore.Read(userId)
		if err != nil {
			return false, err
		}

		for {
			fmt.Printf("Enter mac address: ")
			text, err := GetInput(env.in)
			if err != nil {
				return false, err
			}
			if !IsValidMacAddress(text) {
				fmt.Println("Invalid mac address. Try again")
				continue
			}
			mac := text

			fmt.Printf("Enter alias: ")
			text, err = GetInput(env.in)
			if err != nil {
				return false, err
			}

			client := tplinkapi.Client{
				IP:  ipAddress,
				Mac: mac,
			}
			if client.IsMulticast() {
				return false, fmt.Errorf("multicast addresses not allowed")
			}

			err = env.router.router.MakeIpAddressReservation(client)
			if err != nil {
				return false, err
			}

			alias := text
			device := storage.Device{
				UserId: userId,
				Mac:    client.Mac,
				Alias:  alias,
			}
			err = env.db.DeviceStore.Create(&device)
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
	Action: func(env *Env) (bool, error) {
		deviceId, exists := env.ctx["deviceId"]
		if !exists {
			return false, fmt.Errorf("device id not provided")
		}

		device, err := env.db.DeviceStore.Read(deviceId)
		if err != nil {
			return false, err
		}

		err = env.router.router.DeleteIpAddressReservation(device.Mac)
		if err != nil {
			return false, err
		}

		err = env.db.DeviceStore.Delete(deviceId)
		if err != nil {
			return false, err
		}

		fmt.Println("Device deregistered")
		delete(env.ctx, "deviceId")
		return true, nil
	},
}

var ActionQuit = &Action{
	Name: "Quit",
	Action: func(env *Env) (bool, error) {
		return false, nil
	},
}

func RunMenuActions(env *Env, actions []*Action) (bool, error) {
	if QuitProgram(env.ctx) {
		return true, nil
	}

	var (
		options      strings.Builder
		goBack       bool
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
		fmt.Printf("Choose an action: \n%s\n\nChoice: ", options.String())
		choice, err := GetChoiceInput(env.in, len(actions))
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

		if choice == QuitChoice {
			env.ctx.Set("quit", 1)
			break
		}

		action := actions[choice]
		if action == ActionQuit {
			env.ctx.Set("quit", 1)
			break
		}

		if action.Action != nil {
			goBack, err = action.Action(env)
			if err != nil {
				return false, err
			}
		}

		children := action.GetValidChildren(env.ctx)
		if len(children) > 0 {
			goBack, err = RunMenuActions(env, children)
			if QuitProgram(env.ctx) {
				break
			}

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

func QuitProgram(ctx Context) bool {
	quit := ctx["quit"]
	return quit > 0
}
