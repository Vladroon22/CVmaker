.PHONY:

build: 
	go build -o ./app cmd/main.go

run: build
	./app
