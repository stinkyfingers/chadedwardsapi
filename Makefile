.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: build-lambda
build-lambda:
	GOOS=linux GOARCH=amd64 go build -o lambda-lambda ./lambda
	zip lambda.zip lambda-lambda

.PHONY: deploy
deploy: build-lambda
	cd terraform && terraform apply --var "profile=jds" -auto-approve

.PHONY: plan
plan: build-lambda
	cd terraform && terraform plan --var "profile=jds"

.PHONY: clean
clean:
	rm -f lambda-lambda lambda.zip