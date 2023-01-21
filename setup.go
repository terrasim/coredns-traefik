package traefik_coredns_plugin

import (
	"context"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/docker/docker/client"
	"time"
)

func init() {
	plugin.Register("traefik", setup)
}

func setup(c *caddy.Controller) error {
	traefik, err := parseTraefik(c)
	if err != nil {
		return plugin.Error("traefik", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(handler plugin.Handler) plugin.Handler {
		traefik.Next = handler
		return traefik
	})

	return nil
}

func parseTraefik(c *caddy.Controller) (*Traefik, error) {
	var dockerClient *client.Client
	var err error

	c.Next()
	if c.Next() {
		if dockerClient, err = client.NewClientWithOpts(client.WithHost(c.Val())); err != nil {
			return nil, c.Errf("cannot create docker client: %s", err)
		}
	} else {
		if dockerClient, err = client.NewClientWithOpts(); err != nil {
			return nil, c.Errf("cannot create docker client: %s", err)
		}
	}

	if _, err = dockerClient.Ping(context.Background()); err != nil {
		clog.Errorf("cannot connect to docker daemon: %s", err)
	}

	return &Traefik{
		client:     dockerClient,
		errorCache: NewQueue[string](5*time.Minute, 5, 1*time.Hour),
	}, nil
}
