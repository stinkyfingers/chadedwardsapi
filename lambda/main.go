package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/stinkyfingers/chadedwardsapi/server"
	"github.com/stinkyfingers/lambdify"
)

func main() {
	s, err := server.NewServer("")
	if err != nil {
		log.Fatalln(err)
	}
	mux, err := server.NewMux(s)
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(lambdify.Lambdify(mux))
}
