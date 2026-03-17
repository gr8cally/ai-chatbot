.PHONY: generate ts build run dev clean

deps:
	@npm install


generate: deps
	templ generate

ts: deps
	npx esbuild ts/main.ts --bundle --outfile=static/js/dist/app.js --format=iife --minify

build: ts
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
