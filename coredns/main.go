package main

import (
	_ "github.com/coredns/coredns/core/plugin"
	_ "github.com/terrasim/traefik-coredns-plugin"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

func init() {
	// we must register the traefik directive manually if coredns gets compiled with this file. it gets inserted before
	// the 'forward' directive as it's recommended to be used along forward
	var i int
	for i = 0; i < len(dnsserver.Directives); i++ {
		if dnsserver.Directives[i] == "forward" {
			dnsserver.Directives = append(dnsserver.Directives[:i+1], dnsserver.Directives[i:]...)
			dnsserver.Directives[i] = "traefik"
			return
		}
	}
	dnsserver.Directives = append(dnsserver.Directives, "traefik")
}

func main() {
	coremain.Run()
}
