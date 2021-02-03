SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64

go build -tags netgo -i -v cmd/modbus/main.go -o modbus
upx -9 -k modbus