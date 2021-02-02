SET CGO_ENABLED=0  //不设置也可以，原因不明
SET GOOS=linux
SET GOARCH=amd64

go build -tags netgo -i -v main.go
upx -9 -k main