APP=operator
BIN=$(PWD)/bin/$(APP)
PI_IP=192.168.8.101
GIT_SHA=`git rev-parse --short HEAD`

GO ?= go

pi: clean
	@echo "[pi] Building..."
	@GOOS=linux GOARM=7 GOARCH=arm $(GO) build -o $(BIN)\
		-ldflags "-X main.operatorVersion=$(GIT_SHA) -X github.com/WiseGrowth/go-wisebot/logger.environment=production"

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
