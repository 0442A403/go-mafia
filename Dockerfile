from golang
workdir /app
copy go.mod .
run go mod download
copy . .
run go mod tidy
run go mod download
run go build engine/main.go
run go build client/main.go
