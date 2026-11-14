# The nix dev shell (flake.nix) provides go, golangci-lint, lefthook,
# editorconfig-checker, gitleaks, gosec, govulncheck, trivy, and nodejs;
# every target below runs inside `nix develop`.

.PHONY: build test lint fmt hygiene clean security scan-vuln scan-sast scan-secrets scan-deps setup demo e2e slides

build:
	@mkdir -p backend/bin
	CGO_ENABLED=0 go -C backend build -trimpath -o bin/a2aservers ./cmd/a2aservers
	CGO_ENABLED=0 go -C backend build -trimpath -o bin/worker ./cmd/worker
	CGO_ENABLED=0 go -C backend build -trimpath -o bin/server ./cmd/server

test:
	go -C backend test -race -count=1 ./...

lint:
	go -C backend vet ./...
	cd backend && golangci-lint run ./...
	npm --prefix frontend-react run build

fmt:
	gofmt -s -w backend

hygiene:
	editorconfig-checker

clean:
	rm -rf backend/bin frontend-react/dist frontend-audience/dist coverage.out

# Run all security scanners locally.
security: scan-vuln scan-sast scan-secrets scan-deps

# Go vulnerability database scan.
scan-vuln:
	cd backend && govulncheck ./...

# Static Application Security Testing for Go.
scan-sast:
	cd backend && gosec -quiet ./...

# Secrets detection (scan working tree, not git history).
scan-secrets:
	gitleaks detect --source . --no-git -v

# Dependency license + vulnerability scan.
scan-deps:
	trivy fs --severity HIGH,CRITICAL .

setup:
	./scripts/setup.sh

demo:
	./scripts/run-demo.sh

e2e:
	bash ./scripts/e2e.sh

slides:
	@echo "Opening slides/index.html (also served at http://localhost:8000/slides when backend is running)"
	@command -v open >/dev/null 2>&1 && open slides/index.html || true
