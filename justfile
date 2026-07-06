dev_compose := "-f docker-compose.dev.yml"

default:
    just -l

# Local dev — build and start all services (Postgres + backend + frontend)
up:
    GIT_SHA=$(git rev-parse --short HEAD) docker compose {{ dev_compose }} up --build -d

# Local dev — stop all services (preserves the Postgres volume)
down:
    docker compose {{ dev_compose }} down

# Stop services AND drop the Postgres volume (fresh DB next start)
clean:
    docker compose {{ dev_compose }} down -v

# Follow backend logs (other services are noisier)
logs:
    docker compose {{ dev_compose }} logs -f backend

# Validate backend + frontend build locally before pushing
build:
    cd backend && go build ./...
    cd frontend && npm run build

# Run backend tests (DB-dependent tests skip without TEST_DATABASE_URL)
test:
    cd backend && gofmt -w . && go build ./... && go test ./... && go vet ./... && golangci-lint run

# Run frontend tests
test-frontend:
    cd frontend && npx vitest run && npm test && npm audit

# Run all tests (backend compile-only + frontend)
test-all: test test-frontend

# Generate a random 64-char hex session secret
gensecret:
    cd backend && go run ./cmd/store-cli generate secret

# Generate a random 64-char hex signing key secret
gensigningsecret:
    cd backend && go run ./cmd/store-cli generate signing-secret

# Generate a random 64-char hex email hash salt
gensalt:
    cd backend && go run ./cmd/store-cli generate salt

# Open a psql shell against the dev Postgres
psql:
    docker compose {{ dev_compose }} exec postgres psql -U stevio -d stevio

# Print the magic-link URL from backend stdout (dev mode without SMTP)
magiclink:
    docker compose {{ dev_compose }} logs backend | grep -A12 "mail: not configured" | tail -20
