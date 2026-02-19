.PHONY: dev dev-backend dev-frontend build build-frontend build-backend test test-backend test-frontend docker clean

# Development
dev:
	@echo "Starting development servers..."
	$(MAKE) dev-backend & $(MAKE) dev-frontend & wait

dev-backend:
	go run ./cmd/kafkaui --config config.yaml

dev-frontend:
	cd frontend && npm run dev

# Build
build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build

build-backend:
	CGO_ENABLED=0 go build -o kafkaui ./cmd/kafkaui

# Test
test: test-backend test-frontend

test-backend:
	go test ./... -v

test-frontend:
	cd frontend && npm test

# Docker
docker:
	docker build -t kafkaui .

# Clean
clean:
	rm -f kafkaui
	rm -rf frontend/dist frontend/node_modules
