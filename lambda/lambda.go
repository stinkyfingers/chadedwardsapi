package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/stinkyfingers/chadedwardsapi/server"
	"github.com/stinkyfingers/lambdify"
)

func main() {
	mux, err := server.NewMux()
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(lambdify.Lambdify(mux))
}
