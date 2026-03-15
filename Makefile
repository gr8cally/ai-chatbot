.PHONY: generate ts build run dev clean

deps:
	@command -v templ >/dev/null 2>&1 || go install github.com/a-h/templ/cmd/templ@latest
	@npm install


generate: deps
	templ generate

ts: deps
	npx esbuild ts/main.ts --bundle --outfile=static/js/dist/app.js --format=iife --minify

build: generate ts
	go build -o bin/chatbot .

run: build
	./bin/chatbot

dev:
	@echo "Run in separate terminals:"
	@echo "  templ generate --watch"
	@echo "  npm run watch"
	@echo "  go run ."

clean:
	rm -rf bin/ static/js/dist/ node_modules/
