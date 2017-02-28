APP=operator
BIN=$(PWD)/bin/$(APP)
PI_IP=192.168.8.103

GO ?= go

pi: clean
	@echo "[pi] Building..."
	@sh build_for_production.sh

build b: clean
	@echo "[pi] Building..."
	@$(GO) build -o $(BIN)

run r: build
	@echo "[run] Running..."
	@$(BIN)

clean:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf $(BIN)

upload:
	@echo "[upload] Starting..."
	@scp $(BIN) pi@$(PI_IP):~
	@echo "[upload] Done"

deploy: pi upload
	@echo "[deploy] Done"

.PHONY: pi build b clean upload deploy run
