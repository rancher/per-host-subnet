package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/per-host-subnet/hostnat"
	"github.com/rancher/per-host-subnet/register"
	"github.com/rancher/per-host-subnet/routeupdate"
	"github.com/rancher/per-host-subnet/setting"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "per-host-subnet"
	app.Version = VERSION
	app.Usage = "Support per-host-subnet networking"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			EnvVar: "RANCHER_DEBUG",
		},
		cli.StringFlag{
			Name:   "metadata-address",
			Value:  setting.DefaultMetadataAddress,
			EnvVar: "RANCHER_METADATA_ADDRESS",
		},
		cli.BoolFlag{
			Name:   "enable-route-update",
			EnvVar: "RANCHER_ENABLE_ROUTE_UPDATE",
		},
		cli.StringFlag{
			Name:   "route-update-provider",
			EnvVar: "RANCHER_ROUTE_UPDATE_PROVIDER",
			Value:  setting.DefaultRouteUpdateProvider,
		},
		cli.BoolFlag{
			Name:  "register-service",
			Usage: "Register windows service, invalid for non windows OS.",
		},
		cli.BoolFlag{
			Name:  "unregister-service",
			Usage: "Unregister windows service, invalid for non windows OS.",
		},
	}
	app.Action = appMain
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func appMain(ctx *cli.Context) error {
	if ctx.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if ctx.Bool("register-service") && ctx.Bool("unregister-service") {
		logrus.Fatal("Can not use flag register-service and unregister-service at the same time")
	}
	if err := register.Init(ctx.Bool("register-service"), ctx.Bool("unregister-service")); err != nil {
		return err
	}

	done := make(chan error)

	m, err := metadata.NewClientAndWait(fmt.Sprintf(setting.MetadataURL, ctx.String("metadata-address")))
	if err != nil {
		return errors.Wrap(err, "Failed to create metadata client")
	}

	if ctx.Bool("enable-route-update") {
		_, err := routeupdate.Run(ctx.String("route-update-provider"), m)
		if err != nil {
			return err
		}
	}

	err = hostnat.Watch(m)
	if err != nil {
		return err
	}

	return <-done
}
