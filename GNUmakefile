.PHONY: fmt vet test race build generate generate-check docs docs-check acceptance release-check
fmt:
	gofmt -w main.go internal
vet:
	go vet ./...
test:
	go test ./...
race:
	go test -race ./...
build:
	go build -trimpath -o bin/terraform-provider-qbittorrent .
generate:
	go generate ./internal/client/openapi
generate-check: generate
	git diff --exit-code -- internal/client/openapi/client.gen.go
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate --provider-name qbittorrent
docs-check:
	$(MAKE) docs
	git diff --exit-code -- docs examples
acceptance:
	python3 tests/acceptance/run.py
release-check:
	goreleaser check
	goreleaser release --snapshot --clean --skip=publish,sign
