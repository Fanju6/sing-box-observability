package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Fanju6/sing-box-observability/src/server/internal/fakeupstream"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:9090", "listen address")
	scenario := flag.String("scenario", "online", "online|zero|reset|stale|401|malformed|malformed-json|sensitive|reconnect")
	flag.Parse()
	log.Printf("fake upstream listening on %s scenario=%s", *addr, *scenario)
	if err := http.ListenAndServe(*addr, fakeupstream.New(fakeupstream.ParseScenario(*scenario)).Handler()); err != nil {
		log.Fatal(err)
	}
}
