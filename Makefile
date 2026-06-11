.PHONY: all build test bench run clean token lint e2e

GO      := go
CARGO   := cargo
AUTH    := auth-service
POLICY  := policy-engine
PORT    := 8080

all: build

build: build-go build-rust

build-go:
	cd $(AUTH) && $(GO) build ./...

build-rust:
	$(CARGO) build --release --manifest-path $(POLICY)/Cargo.toml

test: test-go test-rust

test-go:
	cd $(AUTH) && $(GO) test ./...

test-rust:
	$(CARGO) test --manifest-path $(POLICY)/Cargo.toml

bench:
	$(CARGO) bench --manifest-path $(POLICY)/Cargo.toml

run:
	cd $(AUTH) && $(GO) run ./cmd/server/

token:
	cd $(AUTH) && $(GO) run ./cmd/token/ --role=$(ROLE)

lint:
	cd $(AUTH) && $(GO) vet ./...

e2e:
	@echo "=== Starting server ==="
	cd $(AUTH) && $(GO) run ./cmd/server/ &
	@sleep 2
	$(eval ADMIN := $(shell cd $(AUTH) && $(GO) run ./cmd/token/ --role=admin --sub=e2e-admin 2>/dev/null))
	$(eval VIEWER := $(shell cd $(AUTH) && $(GO) run ./cmd/token/ --role=viewer --sub=e2e-viewer 2>/dev/null))
	$(eval DEVELOPER := $(shell cd $(AUTH) && $(GO) run ./cmd/token/ --role=developer --sub=e2e-dev 2>/dev/null))
	@echo ""
	@echo "=== Health check ==="
	@curl -s localhost:$(PORT)/health | python3 -m json.tool 2>/dev/null || curl -s localhost:$(PORT)/health
	@echo ""
	@echo "=== /whoami (admin) ==="
	@curl -s -H "Authorization: Bearer $(ADMIN)" localhost:$(PORT)/whoami | python3 -m json.tool 2>/dev/null || curl -s -H "Authorization: Bearer $(ADMIN)" localhost:$(PORT)/whoami
	@echo ""
	@echo "=== GET /projects (admin — should succeed) ==="
	@curl -s -X GET -H "Authorization: Bearer $(ADMIN)" localhost:$(PORT)/projects
	@echo ""
	@echo "=== POST /projects (viewer — should fail 403) ==="
	@curl -s -X POST -H "Authorization: Bearer $(VIEWER)" localhost:$(PORT)/projects
	@echo ""
	@echo "=== DELETE /projects (developer — should fail 403) ==="
	@curl -s -X DELETE -H "Authorization: Bearer $(DEVELOPER)" localhost:$(PORT)/projects
	@echo ""
	@echo "=== DELETE /projects (admin — should succeed) ==="
	@curl -s -X DELETE -H "Authorization: Bearer $(ADMIN)" localhost:$(PORT)/projects
	@echo ""
	@echo "=== /whoami (no token — should fail 401) ==="
	@curl -s localhost:$(PORT)/whoami
	@echo ""
	@echo "=== Stopping server ==="
	@kill %1 2>/dev/null || true

clean:
	rm -f $(AUTH)/audit.log $(AUTH)/server
	$(CARGO) clean --manifest-path $(POLICY)/Cargo.toml
