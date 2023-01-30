package traefik_coredns_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/miekg/dns"
	"net"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var log = clog.NewWithPlugin("traefik")

type Traefik struct {
	Next plugin.Handler

	client *client.Client

	// errorCache caches how often in a specific time the same error got printed. If it got printed too much, the error
	// output will pause. This is helpful if some kind of docker error (e.g. the docker socket cannot be reached) occurs
	// on every request. This will keep the output significantly more clean.
	errorCache *IncCacheQueue[string]
}

func (t *Traefik) Name() string {
	return "traefik"
}

func (t *Traefik) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	container, err := t.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		format := fmt.Sprintf("failed to get running containers: %s", err)
		logError(t, err, format)
		return t.Next.ServeDNS(ctx, w, r)
	}

	var ip net.IP
	for _, c := range container {
		for key, label := range c.Labels {
			if key == "coredns.traefik.port" {
				if port, _ := strconv.ParseUint(label, 10, 32); port != 0 {
					for _, network := range c.NetworkSettings.Networks {
						domain := strings.TrimSuffix(state.QName(), ".")

						_, subnet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", network.Gateway, network.IPPrefixLen))
						if err != nil {
							format := fmt.Sprintf("failed to get network of host %s", network.IPAddress)
							logError(t, err, format)
							return t.Next.ServeDNS(ctx, w, r)
						}

						if ok, err := hasDomain(fmt.Sprintf("%s:%d", network.IPAddress, port), domain, subnet); err != nil {
							format := fmt.Sprintf("failed to check if host %s has domain %s: %s", network.IPAddress, domain, err)
							logError(t, err, format)
							return t.Next.ServeDNS(ctx, w, r)
						} else if ok {
							ip = net.ParseIP(network.IPAddress)
							break
						}
					}
				}
				break
			}
		}
	}

	if ip != nil {
		var rr dns.RR
		switch state.QType() {
		case dns.TypeA:
			rr = new(dns.A)
			rr.(*dns.A).Hdr = dns.RR_Header{Name: dns.Fqdn(state.QName()), Rrtype: dns.TypeA, Class: dns.ClassINET}
			rr.(*dns.A).A = ip.To4()
		case dns.TypeAAAA:
			rr = new(dns.AAAA)
			rr.(*dns.AAAA).Hdr = dns.RR_Header{Name: dns.Fqdn(state.QName()), Rrtype: dns.TypeAAAA, Class: dns.ClassINET}
			rr.(*dns.AAAA).AAAA = ip
		}

		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, rr)

		state.SizeAndDo(m)
		m = state.Scrub(m)
		_ = w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	} else {
		return t.Next.ServeDNS(ctx, w, r)
	}
}

var hostRegex = regexp.MustCompile(`Host\(\x60(.+)\x60\)`)

type traefikApiRouter struct {
	Rule string `json:"rule"`
	Name string `json:"name"`
}

func hasDomain(host, domain string, routerSubnet *net.IPNet) (bool, error) {
	resp, err := http.Get("http://" + path.Join(host, "api/http/routers"))
	if err != nil {
		return false, err
	}

	var responseBody []traefikApiRouter
	if err = json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return false, err
	}

	for _, response := range responseBody {
		if matches := hostRegex.FindStringSubmatch(response.Rule); len(matches) >= 2 && matches[1] == domain {
			return inSameNetwork(host, response.Name, routerSubnet)
		}
	}
	return false, nil
}

var ipHttpRegex = regexp.MustCompile(`^https?://(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?(/.+)?$`)

type traefikApiService struct {
	LoadBalancer struct {
		Servers []struct {
			Url string `json:"url"`
		} `json:"servers"`
	} `json:"loadBalancer"`
}

func inSameNetwork(host, serviceName string, routerSubnet *net.IPNet) (bool, error) {
	resp, err := http.Get("http://" + path.Join(host, "api/http/services", serviceName))
	if err != nil {
		return false, err
	}

	var responseBody traefikApiService
	if err = json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return false, err
	}

	for _, server := range responseBody.LoadBalancer.Servers {
		if matches := ipHttpRegex.FindStringSubmatch(server.Url); len(matches) >= 2 {
			ip := net.ParseIP(matches[1])
			if routerSubnet.Contains(ip) {
				return true, nil
			}
		} else {
			return false, fmt.Errorf("failed to get ip address of service %s", serviceName)
		}
	}

	return false, nil
}

func logError(traefik *Traefik, originalError error, format string) {
	if traefik.errorCache.Inc(originalError.Error()) {
		log.Warningf("%s (got this error %d times within the last last %d minutes, suppressing it for the next %d minutes)",
			format,
			int(traefik.errorCache.cacheSize),
			int(traefik.errorCache.cacheDuration.Minutes()),
			int(traefik.errorCache.cacheFullDuration.Minutes()),
		)
	} else if !traefik.errorCache.Full(originalError.Error()) {
		log.Warning(format)
	}
}
