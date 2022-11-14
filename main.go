package main

import (
	"os"

	"github.com/giantas/routerman/storage"
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

	router := NewRouterApi(
		os.Getenv("USERNAME"),
		os.Getenv("PASSWORD"),
		os.Getenv("ADDRESS"),
	)

	env := NewEnv(in, db, router)

	_, err = RunMenuActions(env, actions)
	if err != nil {
		exitWithError(err)
	}
}
