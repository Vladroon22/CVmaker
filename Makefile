.PHONY:

run:
	go build -o ./app cmd/main.go
	./app

run-ssl: 
	openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout Key.key -out cert.crt
	go build -o ./app cmd/main.go
	./app

clean:
	rm cert.crt 
	rm Key.key