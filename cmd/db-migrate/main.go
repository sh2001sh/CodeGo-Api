package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "report pending v2 migrations without applying them")
	bootstrap := flag.Bool("bootstrap", false, "create the legacy base schema before applying v2 migrations; only for a new empty database")
	flag.Parse()

	platformconfig.IsMasterNode = true
	if path := os.Getenv("SQLITE_PATH"); path != "" {
		platformdb.SQLitePath = path
	}
	if err := platformstore.InitPrimaryDB(); err != nil {
		panic(fmt.Errorf("initialize primary database: %w", err))
	}
	defer platformstore.CloseDatabases()
	if *bootstrap {
		if *dryRun {
			panic("--bootstrap cannot be combined with --dry-run")
		}
		if err := platformstore.BootstrapPrimarySchema(); err != nil {
			panic(fmt.Errorf("bootstrap primary schema: %w", err))
		}
	}
	if err := platformstore.ApplyV2Migrations(context.Background(), *dryRun); err != nil {
		panic(err)
	}
	if !*dryRun {
		if err := auditprojection.ApplyReadModelMigrations(context.Background()); err != nil {
			panic(err)
		}
	}
}
