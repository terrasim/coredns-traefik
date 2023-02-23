# coredns-traefik [![License](https://img.shields.io/github/license/terrasim/coredns-traefik?style=flat-square)](https://github.com/terrasim/coredns-traefik/blob/main/LICENSE) [![CI](https://img.shields.io/github/actions/workflow/status/terrasim/coredns-traefik/ci.yml?branch=main&style=flat-square)](https://github.com/terrasim/coredns-traefik/actions/workflows/ci.yml)

## Name

_coredns-traefik_ - A CoreDNS plugin to discover domains by docker hosted traefik instances.

## Description

This [CoreDNS](https://coredns.io/) plugin searches traefik docker containers and dynamically redirects to it if the requested domain is a host of the container.
The traefik setup is the same as you would do it without this coredns plugin.
The only thing you have to add is a single label to the _traefik_ containers you want to dynamically resolve: `coredns.traefik.port=<api port>`.
As value this takes the port under which the traefik instance publishes its api (by default `8080`).

## Syntax

```
traefik [DOCKER_ADDRESS]
```

- **DOCKER_ADDRESS** specifies the address of the docker socket. The default address is `unix:///var/run/docker.sock`.

## Examples

Use the default host and fallback to `8.8.8.8` if the requested domain is not a traefik registered container.
```
. {
  traefik
  forward . 8.8.8.8
}
```

Use tcp/network as docker host and fallback to `/etc/resolv.conf` if the requested domain is not a traefik registered container.
```
. {
  traefik tcp://127.0.0.1:6969
  forward . /etc/resolv.conf
}
```

---

> The following example is a simple demonstration how it could look like in action.

**docker-compose.yml**
```yaml
version: '3'

services:
  coredns:
    image: ghcr.io/terrasim/coredns-traefik
    environment:
      # define the corefile via an environment variable
      COREFILE: |
        . {
          traefik
          forward . 8.8.8.8
        }
    ports:
      # the port 53 is likely used on your host system, you might change them if any problems occurs
      - '53:53'
      - '53:53/udp'
    volumes:
      # we need the docker socket to find traefik docker containers
      - '/var/run/docker.sock:/var/run/docker.sock'
      # alternative to defining the corefile as env variable.
      # mount a directory (`./config`) in this case and place a `Corefile` in it which is then used by coredns
      # - './config:/etc/coredns'

  traefik:
    image: traefik:latest
    command:
      - '--api.insecure=true'
      - '--providers.docker=true'
      - '--providers.docker.exposedbydefault=false'
      - '--entrypoints.web.address=:80'
    ports:
      - '80:80'
      - '8080:8080'
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
    labels:
      - 'coredns.traefik.port=8080'

  # create a dummy alpine container which is the traefik service that can be queried by the plugin
  test-client:
    image: alpine:latest
    command:
      - 'sleep'
      - 'infinity'
    labels:
      - 'traefik.enable=true'
      - 'traefik.http.routers.test.rule=Host(`test.local`)'
      - 'traefik.http.routers.test.entrypoints=web'
      - 'traefik.http.services.test.loadbalancer.server.port=80'
```

With this file you can query the dns server under the port its mapped (in this case `53`).

```shell
$ dig @localhost -p 53 test.local
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for more details.
