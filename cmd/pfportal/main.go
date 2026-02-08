package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthurium-ai/personal-finance/internal/app"
	"github.com/anthurium-ai/personal-finance/internal/db"
	"github.com/anthurium-ai/personal-finance/internal/web"
)

func main() {
	addr := flag.String("addr", ":8787", "listen address")
	dbPath := flag.String("db", app.DefaultDBPath(), "sqlite db path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer d.Close()
	if err := db.Migrate(ctx, d); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tmpl, err := web.LoadTemplates()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "pfportal listening on %s\n", *addr)
	if err := app.Run(ctx, d, tmpl, app.Config{Addr: *addr}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
