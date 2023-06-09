.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: build-lambda
build-lambda:
	./build