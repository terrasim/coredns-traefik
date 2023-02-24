package traefik_coredns_plugin

import (
	"context"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/miekg/dns"
	"strconv"
	"testing"
	"time"
)

var docker *client.Client

func init() {
	var err error
	if docker, err = client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock")); err != nil {
		panic(err)
	}
}

func TestAnswer(t *testing.T) {
	traefikMounts := map[string]string{
		"/var/run/docker.sock": "/var/run/docker.sock",
	}
	traefikPorts := []int{80, 8080}
	traefikLabels := map[string]string{
		"coredns.traefik.port": "8080",
	}
	traefikCmd := []string{
		"--api.insecure=true",
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.web.address=:80",
	}
	traefikId, err := newDockerContainer(context.Background(), "traefik", traefikMounts, traefikPorts, traefikLabels, traefikCmd)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if err = deleteDockerContainer(context.Background(), traefikId); err != nil {
			t.Error(err)
		}
	}()

	alpineLabels := map[string]string{
		"traefik.enable":                                      "true",
		"traefik.http.routers.test.rule":                      "Host(`test.local`)",
		"traefik.http.routers.test.entrypoints":               "web",
		"traefik.http.services.test.loadbalancer.server.port": "80",
	}
	alpineCmd := []string{"sleep", "infinity"}
	alpineId, err := newDockerContainer(context.Background(), "alpine", map[string]string{}, []int{}, alpineLabels, alpineCmd)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if err = deleteDockerContainer(context.Background(), alpineId); err != nil {
			t.Error(err)
		}
	}()

	// sleep to leave docker time to create the containers
	time.Sleep(2 * time.Second)

	traefik := new(Traefik)
	traefik.client = docker

	m := test.Case{
		Qname: "test.local",
		Qtype: dns.TypeA,
	}.Msg()
	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	code, err := traefik.ServeDNS(context.Background(), rec, m)

	if err != nil {
		t.Errorf("code: %d, err: %s", code, err)
	}

	t.Log(rec.Msg.Answer[0].String())
}

func newDockerContainer(ctx context.Context, image string, mounts map[string]string, ports []int, labels map[string]string, cmd []string) (string, error) {
	containerConfig := &container.Config{
		Image:  image,
		Labels: labels,
		Cmd:    cmd,
	}
	hostConfig := &container.HostConfig{
		AutoRemove:   true,
		Mounts:       []mount.Mount{},
		PortBindings: map[nat.Port][]nat.PortBinding{},
	}
	for src, dst := range mounts {
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: src,
			Target: dst,
		})
	}
	for _, port := range ports {
		hostConfig.PortBindings[nat.Port(strconv.Itoa(port))] = []nat.PortBinding{
			{
				HostIP:   "127.0.0.1",
				HostPort: strconv.Itoa(port),
			},
		}
	}
	resp, err := docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "test_"+image)
	if err != nil {
		return "", err
	}

	if err = docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func deleteDockerContainer(ctx context.Context, id string) error {
	return docker.ContainerStop(ctx, id, container.StopOptions{})
}
