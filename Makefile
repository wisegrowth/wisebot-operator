APP=operator
BIN=$(PWD)/bin/$(APP)
PLAY_BIN=$(PWD)/bin/play-$(APP)
PI_IP=192.168.8.103

GO ?= go

pi: clean
	@echo "[pi] Building..."
	@GOOS=linux GOARM=7 GOARCH=arm $(GO) build -o $(BIN)

build b: clean
	@echo "[pi] Building..."
	@$(GO) build -o $(BIN)

build-play bp: clean
	@echo "[pi] Building..."
	@cd cmd/play && $(GO) build -o $(PLAY_BIN)

run r: build
	@echo "[run] Running..."
	@$(BIN)

play p: build-play
	@echo "[run] Running play binary..."
	@$(PLAY_BIN)

clean:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf bin/*

upload:
	@echo "[upload] Starting..."
	@scp $(BIN) pi@$(PI_IP):~

deploy: pi
	@echo "[deploy] Starting..."
	@scp $(BIN) pi@$(PI_IP):~

.PHONY: pi build b clean upload deploy run play build-play
