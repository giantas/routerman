package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/omushpapa/routerman/core"
	"github.com/omushpapa/routerman/storage"
	"github.com/omushpapa/tplinkapi"
)

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

var RootActionManageUsers = &Action{
	Name: "Manage users",
	Children: []*Action{
		ActionRegisterUser,
		ActionListUsers,
	},
}

var ActionRegisterUser = &Action{
	Name: "Register a user",
	Action: func(env *core.Env) (Navigation, error) {
		fmt.Fprintf(env.Out, "Name: ")
		name, err := GetInput(env.In)
		if err != nil {
			return NEXT, err
		}
		if name == "" {
			return NEXT, ErrInvalidInput
		}
		user, err := env.Router.RegisterUser(name)
		if err != nil {
			return NEXT, err
		}
		fmt.Fprintf(env.Out, "user %+v created\n", user)
		return NEXT, nil
	},
}

var ActionListUsers = &Action{
	Name: "List users",
	Children: []*Action{
		ActionListUserBandwidthSlots,
		ActionDeregisterUser,
		ActionListDevices,
	},
	Action: func(env *core.Env) (Navigation, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			users      []storage.User
			err        error
		)

		for {
			if showList {
				users, err = env.Store.UserStore.ReadMany(pageSize, pageNumber)
				if err != nil {
					return NEXT, err
				}

				if len(users) == 0 {
					if pageNumber == 1 {
						fmt.Fprintln(env.Out, "no users found")
						return REPEAT, nil
					} else {
						fmt.Fprintln(env.Out, "no more users found")
					}
				}

				dataRows := make([][]string, len(users))
				for i, user := range users {
					dataRows[i] = []string{user.Name}
				}
				err = PrintTable(env.Out, dataRows, true, 2)
				if err != nil {
					return NEXT, err
				}
			} else {
				fmt.Fprintln(env.Out, "no more users found")
			}

			fmt.Fprintf(env.Out, "\nSelect user by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(env.In)
			if err != nil {
				return NEXT, err
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
				return REPEAT, nil
			default:
				position, err := GetChoice(choice, len(users))
				if err == ErrInvalidChoice {
					fmt.Fprintln(env.Out, "invalid choice. try again")
					showList = false
					continue
				}

				user := users[position]
				userId := user.Id

				fmt.Fprintf(env.Out, "Selected user '%s'\n", user.Name)

				_, err = env.Store.UserStore.Read(userId)
				if err != nil {
					return NEXT, err
				}

				env.Ctx.Set("userId", userId)
				return NEXT, err
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
	Action: func(env *core.Env) (Navigation, error) {
		userId, exists := env.Ctx["userId"]
		if !exists {
			return NEXT, fmt.Errorf("user id not provided")
		}

		var (
			err        error
			slots      []storage.BandwidthSlot
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			choice     string
		)
		fmt.Fprintln(env.Out, "\nListing user slots ")

		for {
			if showList {
				entries, err := env.Router.GetUserSlots(userId, pageSize, pageNumber)
				if err != nil {
					return NEXT, err
				}

				dataRows := make([][]string, len(entries))
				for i, entry := range entries {
					dataRows[i] = []string{
						fmt.Sprintf(
							"%s - %s Up:%d/%d Down:%d/%d [%v]\n",
							entry.StartIp, entry.EndIp, entry.UpMin, entry.UpMax, entry.DownMin, entry.DownMax, entry.Enabled,
						),
					}
				}
				err = PrintTable(env.Out, dataRows, true, 2)
				if err != nil {
					return NEXT, err
				}
			} else {
				fmt.Fprintln(env.Out, "no more slots found")
			}

			fmt.Fprintf(env.Out, "\nSelect slot by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err = GetInput(env.In)
			if err != nil {
				return NEXT, err
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
				return REPEAT, nil
			default:
				position, err := GetChoice(choice, len(slots))
				if err == ErrInvalidChoice {
					fmt.Fprintln(env.Out, "invalid choice. try again")
					showList = false
					continue
				}

				slotId := slots[position].Id
				_, err = env.Store.BandwidthSlotStore.Read(slotId)
				if err != nil {
					return NEXT, err
				}

				env.Ctx.Set("slotId", slotId)
				return NEXT, err
			}
		}
	},
}

var ActionAssignSlot = &Action{
	Name:            "Assign bandwidth slot",
	RequiresContext: []string{"userId"},
	Action: func(env *core.Env) (Navigation, error) {
		userId, exists := env.Ctx["userId"]
		if !exists {
			return NEXT, fmt.Errorf("user id not provided")
		}

		var (
			err           error
			slots         []core.BwSlot
			pageNumber    int  = 1
			pageSize      int  = 5
			showList      bool = true
			choice        string
			useDhcpBounds bool
		)

	BOUNDS_LOOP:
		for {
			fmt.Fprintf(env.Out, "Use DHCP IP bounds (y/n): ")
			input, err := GetCharChoice(env.In, []string{"y", "n"})
			if err != nil {
				if err == ErrInvalidChoice {
					continue
				}
				return NEXT, err
			}
			switch input {
			case "y":
				useDhcpBounds = true
				break BOUNDS_LOOP
			case "n":
				useDhcpBounds = false
				break BOUNDS_LOOP
			default:
				fmt.Fprintln(env.Out, "Invalid choice. Try again")
			}
		}

		for {
			if showList {
				slots, err = env.Router.GetAvailableBandwidthSlots(useDhcpBounds)
				if err != nil {
					return NEXT, err
				}

				if len(slots) == 0 {
					fmt.Fprintln(env.Out, "no slots found")
					nav := NEXT
					if pageNumber == 1 {
						nav = BACK
					}
					return nav, nil
				}

				for i, slot := range slots {
					cap, err := slot.GetCapacity()
					if err != nil {
						return NEXT, err
					}
					fmt.Fprintf(env.Out, "%d: %s - %s [%d]\n", i+1, slot.MinAddress, slot.MaxAddress, cap)
				}
			} else {
				fmt.Fprintln(env.Out, "no more slots found")
			}

			fmt.Fprintf(env.Out, "\nSelect slot by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err = GetInput(env.In)
			if err != nil {
				return NEXT, err
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
				return NEXT, nil
			default:
				position, err := GetChoice(choice, len(slots))
				if err == ErrInvalidChoice {
					return NEXT, fmt.Errorf("invalid choice")
				}
				slot := slots[position]

				fmt.Fprintf(env.Out, "Enter start IP [%s]: ", slot.MinAddress)
				startIPText, err := GetInput(env.In)
				if err != nil {
					return NEXT, err
				}

				capacity, _ := slot.GetCapacity()
				fmt.Fprintf(env.Out, "Enter number of devices [Default %d]: ", capacity)
				num, err := GetIntInput(env.In, capacity)
				if num > capacity || num < 1 {
					return NEXT, fmt.Errorf("invalid number")
				}

				maxDown := 1000
				fmt.Fprintf(env.Out, "Enter max download speed (kbps) [Default %d]: ", maxDown)
				maxDown, err = GetIntInput(env.In, maxDown)
				if err != nil {
					return NEXT, err
				}

				maxUp := 1000
				fmt.Fprintf(env.Out, "Enter max upload speed (kbps) [Default %d]: ", maxUp)
				maxUp, err = GetIntInput(env.In, maxUp)
				if err != nil {
					return NEXT, err
				}

				err = env.Router.AssignSlot(
					userId, slot, startIPText, capacity, maxUp, maxDown,
				)
				if err != nil {
					if err, ok := err.(*core.SoftError); ok {
						fmt.Fprintf(env.Out, err.Error())
						return REPEAT, nil
					} else {
						return NEXT, err
					}
				}
				fmt.Fprintln(env.Out, "Entry created successfully")
				return NEXT, err
			}
		}

	},
}

var ActionDeregisterUser = &Action{
	Name:            "Deregister user",
	RequiresContext: []string{"userId"},
	Action: func(env *core.Env) (Navigation, error) {
		userId, exists := env.Ctx["userId"]
		if !exists {
			return NEXT, fmt.Errorf("user id not provided")
		}
		err := env.Router.DeregisterUser(userId)
		if err != nil {
			return NEXT, err
		}

		fmt.Fprintln(env.Out, "user deleted")
		delete(env.Ctx, "userId")
		return BACK, nil
	},
}

var ActionDeleteSlot = &Action{
	Name: "Delete slot",
	Action: func(env *core.Env) (Navigation, error) {
		slotId, exists := env.Ctx["slotId"]
		if !exists {
			return NEXT, fmt.Errorf("slot id not provided")
		}
		err := env.Router.DeleteSlot(slotId)
		if err != nil {
			return NEXT, err
		}
		fmt.Fprintf(env.Out, "slot deleted successfully")
		return BACK, nil
	},
	RequiresContext: []string{"slotId"},
}

var ActionListAvailableSlots = &Action{
	Name: "List available bandwidth slots",
	Action: func(env *core.Env) (Navigation, error) {
		slots, err := env.Router.GetAvailableBandwidthSlots(true)
		if err != nil {
			return NEXT, err
		}
		dataRows := make([][]string, len(slots))
		for x, slot := range slots {
			cap, err := slot.GetCapacity()
			if err != nil {
				return NEXT, err
			}
			dataRows[x] = []string{
				fmt.Sprintf("%s - %s [%d]", slot.MinAddress, slot.MaxAddress, cap),
			}

		}
		err = PrintTable(env.Out, dataRows, true, 2)
		return NEXT, err
	},
}

var RootActionManageDevices = &Action{
	Name: "Manage devices",
	Children: []*Action{
		ActionListDevices,
		ActionShowConnectedDevices,
		ActionExportARPBindings,
		ActionExportDhcpAddressReservations,
	},
}

var ActionListDevices = &Action{
	Name: "List devices",
	Children: []*Action{
		ActionDeregisterDevice,
	},
	Action: func(env *core.Env) (Navigation, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			devices    []storage.Device
			err        error
		)
		userId, userIdProvided := env.Ctx["userId"]

		for {
			if showList {
				if userIdProvided && userId != 0 {
					devices, err = env.Store.DeviceStore.ReadManyByUserId(userId, pageSize, pageNumber)
				} else {
					devices, err = env.Store.DeviceStore.ReadMany(pageSize, pageNumber)
				}

				if err != nil {
					return NEXT, err
				}
				if len(devices) == 0 {
					fmt.Fprintln(env.Out, "no devices found")
					return NEXT, nil
				}
				dataRows := make([][]string, len(devices))
				for i, device := range devices {
					user, err := device.GetUser(env.Store.UserStore)
					var details string
					if err != nil {
						details = device.Alias
					} else {
						details = fmt.Sprintf("%s\t\t%s", device.Alias, user.Name)
					}
					dataRows[i] = []string{device.Mac, details}
				}
				err = PrintTable(env.Out, dataRows, true, 3)
				if err != nil {
					return NEXT, err
				}
			} else {
				fmt.Fprintln(env.Out, "no more users found")
			}

			fmt.Fprintf(env.Out, "\nSelect device by number or scroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(env.In)
			if err != nil {
				return NEXT, err
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
				return NEXT, nil
			default:
				num, err := GetChoice(choice, len(devices))
				if err == ErrInvalidChoice {
					fmt.Fprintln(env.Out, "invalid choice. try again")
					showList = false
					continue
				}

				deviceId := devices[num].Id
				env.Ctx.Set("deviceId", deviceId)
				return NEXT, nil
			}
		}
	},
}

var ActionShowConnectedDevices = &Action{
	Name: "Show connected devices",
	Action: func(env *core.Env) (Navigation, error) {
		var (
			pageNumber int  = 1
			pageSize   int  = 5
			showList   bool = true
			stats      tplinkapi.ClientStatistics
			err        error
		)

		stats, devices, err := env.Router.GetConnectedDevices()
		if err != nil {
			return NEXT, err
		}

		deviceMap := make(map[string]storage.Device)
		for _, device := range devices {
			deviceMap[device.Mac] = device
		}

		for {
			if showList {
				if len(stats) == 0 {
					if pageNumber == 1 {
						fmt.Fprintln(env.Out, "No connected devices")
						return NEXT, err
					} else {
						fmt.Fprintln(env.Out, "No more devices found")
					}
				}

				dataRows := make([][]string, len(stats))
				for i, stat := range stats {
					device, exists := deviceMap[stat.Mac]
					details := "Unknown"
					if exists {
						user, err := device.GetUser(env.Store.UserStore)
						if err != nil {
							details = device.Alias
						} else {
							details = fmt.Sprintf("%s\t\t%s", device.Alias, user.Name)
						}
					}
					dataRows[i] = []string{stat.IP, stat.Mac, details}
				}
				err = PrintTable(env.Out, dataRows, true, 3)
				if err != nil {
					return NEXT, err
				}
			} else {
				fmt.Fprintln(env.Out, "No more devices found")
			}

			fmt.Fprintf(env.Out, "\nScroll with n(ext)/p(revious)/q(uit): ")
			choice, err := GetInput(env.In)
			if err != nil {
				return NEXT, err
			}

			switch choice {
			case "n":
				if len(stats) == pageSize {
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
				return NEXT, nil
			default:
				fmt.Fprintln(env.Out, "Invalid input")
				continue
			}
		}
	},
}

var ActionExportARPBindings = &Action{
	Name: "Export ARP Bindings",
	Action: func(env *core.Env) (Navigation, error) {
		var (
			bindings []tplinkapi.ClientReservation
			err      error
		)

		bindings, err = env.Router.GetIpMacBindings()
		if err != nil {
			return NEXT, err
		}

		if len(bindings) == 0 {
			fmt.Fprintln(env.Out, "No bindings found")
			return NEXT, nil
		}

		filename := "bindings.csv"
		err = ExportBindings(bindings, filename)
		fmt.Fprintf(env.Out, "saved to '%s'\n", filename)
		return NEXT, err
	},
}

var ActionExportDhcpAddressReservations = &Action{
	Name: "Export DHCP Address Reservations",
	Action: func(env *core.Env) (Navigation, error) {
		var (
			reservations []tplinkapi.ClientReservation
			err          error
		)

		reservations, err = env.Router.GetAddressReservations()
		if err != nil {
			return NEXT, err
		}

		if len(reservations) == 0 {
			fmt.Fprintln(env.Out, "No reservations found")
			return NEXT, nil
		}

		filename := "reservations.csv"
		err = ExportBindings(reservations, filename)
		fmt.Fprintf(env.Out, "saved to '%s'\n", filename)
		return NEXT, err
	},
}

var ActionRegisterDevice = &Action{
	Name:            "Register a device",
	RequiresContext: []string{"userId", "slotId"},
	Action: func(env *core.Env) (Navigation, error) {
		userId, exists := env.Ctx["userId"]
		if !exists {
			return NEXT, fmt.Errorf("user id not provided")
		}

		slotId, exists := env.Ctx["slotId"]
		if !exists {
			return NEXT, fmt.Errorf("slot id not provided")
		}

		for {
			fmt.Fprintf(env.Out, "Enter mac address: ")
			mac, err := GetInput(env.In)
			if err != nil {
				return NEXT, err
			}
			if !IsValidMacAddress(mac) {
				fmt.Fprintln(env.Out, "Invalid mac address. Try again")
				continue
			}

			fmt.Fprintf(env.Out, "Enter alias: ")
			alias, err := GetInput(env.In)
			if err != nil {
				return NEXT, err
			}

			err = env.Router.RegisterDevice(mac, alias, slotId, userId)
			if err != nil {
				return NEXT, err
			}

			break
		}
		return NEXT, nil
	},
}

var ActionDeregisterDevice = &Action{
	Name:            "Deregister device",
	RequiresContext: []string{"deviceId"},
	Action: func(env *core.Env) (Navigation, error) {
		deviceId, exists := env.Ctx["deviceId"]
		if !exists {
			return NEXT, fmt.Errorf("device id not provided")
		}

		err := env.Router.DeregisterDevice(deviceId)
		if err != nil {
			return NEXT, err
		}

		fmt.Fprintln(env.Out, "Device deregistered")
		delete(env.Ctx, "deviceId")
		return BACK, nil
	},
}

var RootActionManageInternetAccess = &Action{
	Name: "Manage internet access",
	Children: []*Action{
		ActionShowConnectedDevices,
		ActionListBlockedDevices,
		ActionBlockDevice,
		ActionUnblockDevice,
	},
}

var ActionListBlockedDevices = &Action{
	Name: "Show blocked devices",
	Action: func(env *core.Env) (Navigation, error) {
		devices, err := env.Router.GetBlockedDevices()
		if err != nil {
			return NEXT, err
		}

		if len(devices) == 0 {
			fmt.Fprintln(env.Out, "no blocked devices found")
			return NEXT, nil
		}

		fmt.Fprintln(env.Out, "Blocked devices:")
		dataRows := make([][]string, len(devices))
		for i, device := range devices {
			user, err := device.GetUser(env.Store.UserStore)
			var details string
			if err != nil {
				details = device.Alias
			} else {
				details = fmt.Sprintf("%s\t\t%s", device.Alias, user.Name)
			}
			dataRows[i] = []string{device.Alias, device.Mac, details}
		}
		err = PrintTable(env.Out, dataRows, true, 3)
		return NEXT, err
	},
}

var ActionBlockDevice = &Action{
	Name: "Block device",
	Action: func(env *core.Env) (Navigation, error) {
		fmt.Fprintf(env.Out, "Enter devic mac address: ")
		mac, err := GetInput(env.In)
		if err != nil {
			return NEXT, nil
		}

		err = env.Router.BlockDevice(mac)
		return NEXT, err
	},
}

var ActionUnblockDevice = &Action{
	Name: "Unblock device",
	Action: func(env *core.Env) (Navigation, error) {
		fmt.Fprintf(env.Out, "Enter devic mac address: ")
		mac, err := GetInput(env.In)
		if err != nil {
			return NEXT, nil
		}

		err = env.Router.UnblockDevice(mac)
		if err != nil {
			return NEXT, err
		}

		fmt.Fprintf(env.Out, "device '%s' unblocked", mac)
		return NEXT, err
	},
}

var ActionQuit = &Action{
	Name: "Quit",
	Action: func(env *core.Env) (Navigation, error) {
		return NEXT, nil
	},
}

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

func QuitProgram(ctx core.Context) bool {
	quit := ctx["quit"]
	return quit > 0
}

func ExportBindings(bindings []tplinkapi.ClientReservation, filename string) error {
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].IpAsInt() < bindings[j].IpAsInt()
	})

	csvData := make([][]string, len(bindings)+1)
	headers := []string{"Mac", "IP", "Enabled"}
	csvData[0] = headers

	for i, binding := range bindings {
		enabled := "n"
		if binding.Enabled {
			enabled = "y"
		}

		csvData[i+1] = []string{binding.Mac, binding.IP, enabled}
	}

	if err := WriteToCsv(filename, csvData); err != nil {
		return err
	}

	return nil
}
