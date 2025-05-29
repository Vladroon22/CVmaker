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
	sudo docker compose up --build -d

compose-rmi:
	sudo docker rmi cvmaker-cvmake

compose-stop:
	sudo docker compose down

compose-delete:
	sudo docker compose down -v
	sudo docker rmi cvmaker-cvmake
	sudo docker rmi postgres:16
	sudo docker rmi redis