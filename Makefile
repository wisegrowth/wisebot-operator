APP=operator
BIN=$(PWD)/bin/$(APP)
PI_IP=192.168.8.101

pi: clean
	@echo "[pi] Building..."
	@GOOS=linux GOARCH=arm go build -o $(BIN)

clean:
	@echo "[clean] Removing $(BIN)..."
	@rm -rf bin/*

upload: pi
	@echo "[upload] Starting..."
	@scp $(BIN) pi@$(PI_IP):~

