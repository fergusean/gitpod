package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/events"
	"github.com/containerd/typeurl"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "export",
				Usage: "Starts exporting metrics",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:      "socket",
						TakesFile: true,
						Usage:     "path to the containerd socket",
						Required:  true,
					},
				},
				Action: func(c *cli.Context) error {
					return serveContainerdMetrics(c.String("socket"))
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type prep struct {
	T      time.Time
	Parent string
	Name   string
}
type commit struct {
	Dur  time.Duration
	Name string
}

var (
	prepByKey   = make(map[string]prep)
	commitByKey = make(map[string]commit)
)

func serveContainerdMetrics(fn string) error {
	client, err := containerd.New(fn, containerd.WithDefaultNamespace("k8s.io"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	evts, errs := client.EventService().Subscribe(ctx)
	for {
		var e *events.Envelope
		select {
		case err := <-errs:
			return err
		case <-sigChan:
			return nil
		case e = <-evts:
		}

		evt, err := typeurl.UnmarshalAny(e.Event)
		if err != nil {
			logrus.WithError(err).Warn("skipping event")
			continue
		}

		switch evt := evt.(type) {
		case *apievents.SnapshotPrepare:
			prepByKey[evt.Key] = prep{
				T:      time.Now(),
				Parent: evt.Parent,
			}
		case *apievents.SnapshotCommit:
			p, ok := prepByKey[evt.Key]
			if ok {
				commitByKey[evt.Key] = commit{
					Dur:  time.Since(p.T),
					Name: evt.Name,
				}
			}
		case *apievents.ContainerCreate:
			pushContainerMetrics(evt.ID, evt.Image)
		}
	}

	return nil
}

func pushContainerMetrics(id, image string) {

}
