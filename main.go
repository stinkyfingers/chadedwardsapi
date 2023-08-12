package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/stinkyfingers/chadedwardsapi/server"
)

const (
	port = ":8087"
)

func main() {
	flag.Parse()
	fmt.Print("Running. \n")
	s, err := server.NewServer("jds")
	if err != nil {
		log.Fatalln(err)
	}
	rh, err := server.NewMux(s)
	if err != nil {
		log.Fatal(err)
	}

	err = http.ListenAndServe(port, rh)
	if err != nil {
		log.Print(err)
	}

}
