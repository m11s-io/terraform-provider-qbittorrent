package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/provider"
	"log"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Run with debugger support")
	flag.Parse()
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{Address: "registry.terraform.io/m11s-io/qbittorrent", Debug: *debug}); err != nil {
		log.Fatal(err)
	}
}
