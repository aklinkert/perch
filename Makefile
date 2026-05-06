.PHONY: dev build test tidy docker

dev:
	@echo "Starting Tailwind watcher and dev server..."
	npx tailwindcss -i web/tailwind.css -o web/static/app.css --watch &
	TAILWIND_PID=$$!; \
	trap "kill $$TAILWIND_PID 2>/dev/null" EXIT INT TERM; \
	go run ./cmd/dashboard run --config config.yaml

build:
	npx tailwindcss -i web/tailwind.css -o web/static/app.css --minify
	mkdir -p dist
	go build -trimpath -ldflags='-s -w' -o dist/dashboard ./cmd/dashboard

test:
	go test ./...

tidy:
	go mod tidy

docker:
	docker build -t work-dashboard .
