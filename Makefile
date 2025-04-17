build-mac:
	rm -f app/resources/enchanted-twin
	cd app/ && pnpm install
	cd backend/golang && make release
	mkdir -p app/resources
	cp backend/golang/bin/enchanted-twin-darwin-arm64 app/resources/enchanted-twin
	chmod +x app/resources/enchanted-twin
	cd app && pnpm build:mac

.PHONY: build-mac