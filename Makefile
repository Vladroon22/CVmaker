.PHONY:

ssl:
	openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout Key.key -out cert.crt
clean:
	rm cert.crt
	rm Key.key

run:
	go build -o ./app cmd/main.go
	./app

compose-run:
	docker compose up --build -d

compose-stop:
	docker compose down

compose-delete:
	docker compose down -v
	docker rmi cvmaker-cvmake
	docker rmi postgres:16
	docker rmi redis
