.PHONY:


ssl: 
	openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout Key.key -out cert.crt
clean:
	rm cert.crt 
	rm Key.key

run:
	go build -o ./app cmd/main.go
	./app

make image:
	sudo docker build . -t cvmaker

make docker:
	sudo docker network create my-net
	sudo docker run --name=cvmake -p 8080:8080 --network my-net -d cvmaker
	sudo docker run --name=CV -e POSTGRES_PASSWORD=55555 -e POSTGRES_DB=CV -p 5432:5432 --network my-net -d postgres:16.2
	sudo docker run --name=MyRedis -p 6379:6379 --network my-net -d redis

make docker-rm:
	sudo docker rm -f cvmake
	sudo docker rm -f CV
	sudo docker rm -f MyRedis
	sudo docker network rm my-net