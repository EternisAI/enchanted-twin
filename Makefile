build-mac:
	rm app/resources/enchanted-twin
	cd backend/golang && make build
	mkdir -p app/resources
	cp backend/golang/bin/enchanted-twin app/resources/
	cd app && pnpm build:mac

.PHONY: build-mac