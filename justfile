dev:
    ls **.go **.js | entr -r godotenv go run .

build:
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE" CC=musl-gcc go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./jingle
