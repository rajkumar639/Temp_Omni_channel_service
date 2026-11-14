package main

import (
	"fmt"
	"os"

	"omniChannelservice/apigateway"
	"omniChannelservice/db"
	"omniChannelservice/integration"
	"omniChannelservice/router"
	"omniChannelservice/service"
	"omniChannelservice/tomlloader"
	"omniChannelservice/util"
)

func main() {
	// ── 1. Load configuration from toml/ directory ────────────────────────
	cfg, err := tomlloader.Load("toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Initialise logger (writes to logs/app.log + stdout) ───────────
	log, err := util.NewLogger(cfg.App.Log.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init error: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	log.Info("=== %s v%s starting (env=%s) ===",
		cfg.App.App.Name, cfg.App.App.Version, cfg.App.App.Env)

	// ── 3. Connect to MSSQL ───────────────────────────────────────────────
	if err := db.Init(cfg.DB.Database); err != nil {
		log.Warn("MSSQL unavailable, continuing without DB: %v", err)
	} else {
		defer db.Close()
		log.Info("database ready")
	}

	// ── 4. Build async notification service ──────────────────────────────
	comm := integration.NewCommunicator(cfg.Communication, log)
	notifier := service.NewNotificationService(comm, log)
	defer notifier.Close()

	// ── 5. Build router from routes.toml ──────────────────────────────────
	mux := router.New(cfg.Routes.Routes, log, notifier)

	// ── 6. Assemble and start API gateway ─────────────────────────────────
	addr := fmt.Sprintf("%s:%d", cfg.App.Server.Host, cfg.App.Server.Port)
	gw := apigateway.New(addr, mux, log)

	if err := gw.Start(); err != nil {
		log.Fatal("gateway error: %v", err)
	}
}
