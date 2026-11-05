package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"messwa/internal/config"
)

func fetchPortalQR(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("fetch-qr", flag.ContinueOnError)
	out := fs.String("out", filepath.Join(cfg.StateDir, "current-qr.png"), "private PNG output path")
	user := fs.String("user", "", "fetch with this account's mess API key; defaults to the first admin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	c, err := userPortal(ctx, cfg, *user)
	if err != nil {
		return err
	}
	image, err := c.QR(ctx)
	if err != nil {
		return err
	}
	// Replace via a private temporary file so an existing permissive file does
	// not retain its permissions and failed writes cannot leave half a QR.
	f, err := os.CreateTemp(filepath.Dir(*out), ".mess-qr-*.png")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(image.PNG); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, *out); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Current portal QR saved to %s. It becomes invalid when the portal resets the token.\n", *out)
	return nil
}
