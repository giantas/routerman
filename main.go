package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/giantas/routerman/storage"
	"github.com/giantas/tplinkapi"
)

func main() {
	in := os.Stdin
	cfg := storage.DbConfig{
		Init: false,
		URI:  "routerman.db",
	}
	db, err := storage.ConnectDatabase(cfg)
	if err != nil {
		exitWithError(err)
	}
	defer db.Close()

	actions := []*Action{
		RootActionManageUsers,
		RootActionManageDevices,
		ActionQuit,
	}

	service := tplinkapi.RouterService{
		Username: os.Getenv("USERNAME"),
		Password: os.Getenv("PASSWORD"),
		Address:  os.Getenv("ADDRESS"),
	}

	env := NewEnv(in, db, service)

	_, err = RunMenuActions(env, actions)
	if err != nil {
		exitWithError(err)
	}
}

type Env struct {
	in     io.Reader
	db     *storage.Store
	ctx    Context
	router tplinkapi.RouterService
}

func NewEnv(in io.Reader, db *sql.DB, router tplinkapi.RouterService) *Env {
	store := storage.NewStore(db)

	return &Env{
		in:     in,
		db:     store,
		ctx:    make(Context),
		router: router,
	}
}

func routerFunctions() {
	service := tplinkapi.RouterService{
		Username: os.Getenv("USERNAME"),
		Password: os.Getenv("PASSWORD"),
		Address:  os.Getenv("ADDRESS"),
	}

	routerInfo, err := service.GetRouterInfo()
	if err != nil {
		exitWithError(err)
	}
	fmt.Printf("Info: %+v\n", routerInfo)

	// clientInfo, err := service.GetClientInfo()
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf("Info: %+v\n", clientInfo)

	// stats, err := service.GetStatistics()
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf("%d devices connected:\n", len(stats))
	// for _, client := range stats {
	// 	fmt.Printf("IP: %s Mac: %s Usage: %f\n", client.IP, client.Mac, client.BytesIn(MB))
	// }

	// reservations, err := service.GetAddressReservations()
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf("%d reservations made\n", len(reservations))
	// for _, r := range reservations {
	// 	fmt.Printf("Id: %d IP: %s Mac: %s Enabled: %v\n", r.Id, r.IP, r.Mac, r.Enabled)
	// }

	// reservations, err := service.GetIpMacBindings()
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf("%d reservations made\n", len(reservations))
	// for _, r := range reservations {
	// 	fmt.Printf("Id: %d IP: %s Mac: %s Enabled: %v\n", r.Id, r.IP, r.Mac, r.Enabled)
	// }

	// client := Client{
	// 	IP:  "192.168.0.186",
	// 	Mac: "F2:28:A9:A4:75:6C",
	// }
	// err := service.MakeIpAddressReservation(client)
	// if err != nil {
	// 	exitWithError(err)
	// }

	// err := service.DeleteIpAddressReservation(client.Mac)
	// if err != nil {
	// 	exitWithError(err)
	// }

	// bwControl, err := service.GetBandwidthControlDetails()
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf(
	// 	"Bandwidth control status: %v upTotal: %d downTotal: %d \nEntries: %d\n",
	// 	bwControl.Enabled, bwControl.UpTotal, bwControl.DownTotal, len(bwControl.Entries),
	// )
	// for _, entry := range bwControl.Entries {
	// 	fmt.Printf(
	// 		"IP: %s-%s MinUp: %d MaxUp: %d MinDown: %d MaxDown: %d Enabled: %v\n",
	// 		entry.StartIp, entry.EndIp, entry.UpMin, entry.UpMax, entry.DownMin, entry.DownMax, entry.Enabled,
	// 	)
	// }

	// config := BandwidthControlDetail{
	// 	Enabled:   true,
	// 	UpTotal:   80000,
	// 	DownTotal: 80000,
	// }
	// err := service.ToggleBandwidthControl(config)
	// if err != nil {
	// 	exitWithError(err)
	// }

	// entry := BandwidthControlEntry{
	// 	Enabled: true,
	// 	StartIp: "192.168.0.251",
	// 	EndIp:   "192.168.0.254",
	// 	UpMin:   100,
	// 	UpMax:   150,
	// 	DownMin: 100,
	// 	DownMax: 150,
	// }
	// id, err := service.AddBwControlEntry(entry)
	// if err != nil {
	// 	exitWithError(err)
	// }
	// fmt.Printf("Entry added with id %d\n", id)

	// err := service.DeleteBwControlEntry(15)
	// if err != nil {
	// 	exitWithError(err)
	// }

	// err = service.Logout()
	// if err != nil {
	// 	exitWithError(err)
	// }
}
