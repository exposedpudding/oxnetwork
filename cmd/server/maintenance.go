package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/backup"
	"github.com/exposedpudding/oxnetwork/internal/config"
)

const maintenanceUsage = `oxnet-work maintenance commands (ADR-016):

  oxnet-work backup  [-config FILE] [-db FILE] DEST    write a verified, session-free copy of the database
  oxnet-work verify  BACKUP                            check a backup (integrity, schema version, .sha256 file)
  oxnet-work restore [-config FILE] [-db FILE] [-force] BACKUP
                                                       replace the database with a backup; STOP THE APP FIRST

The database location comes from -db, else the config file, else OXNET_DATA_DIR.
Results are printed as JSON; the exit status is non-zero on any failure.`

// runMaintenance handles the backup, verify and restore subcommands. It reports
// whether args named one, so main can fall through to starting the server.
func runMaintenance(args []string, stdout, stderr io.Writer) (handled bool, code int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case "backup", "verify", "restore":
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, maintenanceUsage)
		return true, 0
	default:
		return false, 0
	}
	fail := func(format string, a ...any) (bool, int) {
		fmt.Fprintf(stderr, "error: "+format+"\n", a...)
		return true, 1
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", os.Getenv("OXNET_CONFIG"), "YAML configuration file")
	dbFlag := flags.String("db", "", "database file (default: from config or OXNET_DATA_DIR)")
	force := flags.Bool("force", false, "restore: replace an existing database (it is saved as a safety copy first)")
	if err := flags.Parse(args[1:]); err != nil {
		return true, 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, maintenanceUsage)
		return true, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	target := flags.Arg(0)

	dbPath := *dbFlag
	if dbPath == "" && args[0] != "verify" {
		resolved, err := config.DatabasePathFor(*configPath)
		if err != nil {
			return fail("%v", err)
		}
		dbPath = resolved
	}
	var result any
	var err error
	switch args[0] {
	case "backup":
		result, err = backup.Create(ctx, dbPath, target)
	case "verify":
		result, err = backup.Verify(ctx, target)
	case "restore":
		result, err = backup.Restore(ctx, target, dbPath, backup.RestoreOptions{Force: *force})
	}
	if err != nil {
		if args[0] == "restore" && errors.Is(err, backup.ErrTargetExists) {
			return fail("%v (pass -force to replace %s; the current database is kept as a safety copy)", err, dbPath)
		}
		return fail("%v", err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fail("%v", err)
	}
	return true, 0
}
