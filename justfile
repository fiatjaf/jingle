dev:
    ls **.go **.js | entr -r godotenv go run .

build:
    CC=musl-gcc go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./relay29
