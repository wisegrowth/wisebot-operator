APP=operator
BIN=$(PWD)/bin/$(APP)
PI_IP=192.168.0.30

GO ?= go

pi: build_for_production.sh clean
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

.PHONY: pi build b clean upload deploy run
