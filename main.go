package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/stinkyfingers/chadedwardsapi/server"
)

const (
	port = ":8088"
)

func main() {
	flag.Parse()
	fmt.Print("Running. \n")
	rh, err := server.NewMux()
	if err != nil {
		log.Fatal(err)
	}

	err = http.ListenAndServe(port, rh)
	if err != nil {
		log.Print(err)
	}

}
