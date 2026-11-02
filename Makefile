.PHONY: web test build run db-up

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/launchcrew?sslmode=disable
export DATABASE_URL

# Local Postgres for the db tests; matches the URL above.
db-up:
	@docker start launchcrew-pg >/dev/null 2>&1 || docker run -d --name launchcrew-pg \
		-e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=launchcrew -p 5432:5432 postgres:16 >/dev/null
	@until docker exec launchcrew-pg pg_isready -U postgres -d launchcrew >/dev/null 2>&1; do sleep 1; done

web:
	cd web && npm ci && npm run build

test: web
	go test -p 1 ./...
	cd web && npm test

build: web
	go build -o bin/launchcrew ./cmd/launchcrew

run: build
	./bin/launchcrew
