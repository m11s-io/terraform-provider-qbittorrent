.PHONY: fmt vet test race build docs docs-check acceptance release-check
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
