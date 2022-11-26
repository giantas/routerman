package cmd

import (
	"fmt"
	"os"

	"github.com/omushpapa/routerman/cli"
	"github.com/omushpapa/routerman/storage"
	"github.com/spf13/cobra"
)

var (
	initDb bool
)

var rootCmd = &cobra.Command{
	Use:   "routerman",
	Short: "TP-Link Router Interface",
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database manager",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := storage.DbConfig{
			Init: initDb,
			URI:  "routerman.db",
		}
		db, err := storage.ConnectDatabase(cfg)
		if err != nil {
			exitWithError(err)
		}
		defer db.Close()
	},
}

var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "CLI Interface",
	Run: func(cmd *cobra.Command, args []string) {
		in := os.Stdin
		cfg := storage.DbConfig{
			Init: initDb,
			URI:  "routerman.db",
		}
		db, err := storage.ConnectDatabase(cfg)
		if err != nil {
			exitWithError(err)
		}
		defer db.Close()

		actions := []*cli.Action{
			cli.RootActionManageUsers,
			cli.RootActionManageDevices,
			cli.RootActionManageInternetAccess,
			cli.ActionQuit,
		}

		router := cli.NewRouterApi(
			os.Getenv("USERNAME"),
			os.Getenv("PASSWORD"),
			os.Getenv("ADDRESS"),
		)

		env := cli.NewEnv(in, db, router)

		_, err = cli.RunMenuActions(env, actions)
		if err != nil {
			exitWithError(err)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.Flags().BoolVar(&initDb, "init", false, "Initialise the database")

	rootCmd.AddCommand(cliCmd)
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, "error: ", err.Error())
	os.Exit(1)
}
