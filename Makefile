.PHONY: build test web web-dev searoute

build: ## Build Go packages and the web page
	go build ./...
	cd web && npm run build

test: ## Go tests and the page's replay tests
	go test ./...
	cd web && npm test

web-dev:
	cd web && npm run dev

searoute: ## Build the seaRoute submodule
	cd third_party/seaRoute && go build ./...
