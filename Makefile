APP=operator
BIN=$(PWD)/bin/$(APP)
PI_IP=192.168.8.101

GO ?= go

pi: clean
	@echo "[pi] Building..."
	@GOOS=linux GOARCH=arm $(GO) build -o $(BIN)

build b: clean
	@echo "[pi] Building..."
	@$(GO) build -o $(BIN)

clean:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf bin/*

upload:
	@echo "[upload] Starting..."
	@scp $(BIN) pi@$(PI_IP):~

deploy: pi
	@echo "[deploy] Starting..."
	@scp $(BIN) pi@$(PI_IP):~

.PHONY: pi build b clean upload deploy
