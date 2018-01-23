APP=operator
UPLOADER_APP=uploader
BIN_FOLDER=$(PWD)/bin
BIN=$(BIN_FOLDER)/$(APP)
UPLOADER_BIN=$(BIN_FOLDER)/$(UPLOADER_APP)

VERSION=1.1.99
PI_IP=192.168.8.101
GIT_SHA=`git rev-parse --short HEAD`
BASE_URL=https://s3.us-west-2.amazonaws.com/wisebot-operator-releases/operator-
REGION=us-west-2
BUCKET_RELEASES=wisebot-operator-releases

OPENSSL ?= openssl
GO ?= go

pi: clean
	@echo "[pi] Building..."
	@GOOS=linux GOARM=7 GOARCH=arm $(GO) build -o $(BIN)-$(VERSION)\
		-ldflags "-X main.operatorVersion=$(GIT_SHA) -X github.com/WiseGrowth/go-wisebot/logger.environment=production -X main.version=$(VERSION) -X main.baseURL=$(BASE_URL)"

build b: clean
	@echo "[pi] Building..."
	@$(GO) build -o $(BIN)-$(VERSION)-darwin \
		-ldflags "-X main.operatorVersion=$(GIT_SHA) -X github.com/WiseGrowth/go-wisebot/logger.environment=production -X main.version=$(VERSION) -X main.baseURL=$(BASE_URL)"

build-uploader:
	@$(GO) build -o $(BIN_FOLDER)/$(UPLOADER_APP)-darwin \
		-ldflags "-X main.filename=$(APP) -X main.filepath=$(BIN_FOLDER) -X main.bucket=$(BUCKET_RELEASES) -X main.region=$(REGION)" $(PWD)/$(UPLOADER_APP)/main.go

run r: build
	@echo "[run] Running..."
	@$(BIN)

clean:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf $(BIN)

clean-all:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf $(BIN_FOLDER)/*

upload:
	@echo "[upload] Starting..."
	@cp $(BIN)-$(VERSION) $(BIN)
	@scp $(BIN) pi@$(PI_IP):~
	@rm $(BIN)
	@echo "[upload] Done"

deploy: pi upload

checksum:
	@echo "[Checksum] Generating $(APP) Checksum..."
	@$(OPENSSL) sha256 $(BIN)-$(VERSION) | awk '{print $$2}' > $(BIN)-$(VERSION).checksum

new-release: pi checksum
	@echo "[New Release] Uploading new release to AWS S3 Bucket..."
	@$(UPLOADER_BIN)-darwin $(VERSION)

.PHONY: pi build b clean upload deploy run checksum new-release clean-all
