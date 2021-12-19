test:
	@go test -v -count=1 ./...

run:
	@go run ./cmd/bake/ ./testdata/1.bake
	@go run ./cmd/bake/ ./testdata/2.bake

verbose:
	@go run . -v ./testdata/1.bake
