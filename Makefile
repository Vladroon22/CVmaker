.PHONY:

build-ssl: 
	openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout Key.key -out cert.crt
	go build -o ./app cmd/main.go

build:
	go build -o ./app cmd/main.go

run: build
	./app
