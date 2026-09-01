.PHONY: test test-race build clean run-worker run-starter docker-up docker-down

# Run unit tests only
test:
	go test -v ./...

test-race:
	go test -v -race ./...

build:
	mkdir -p bin
	go build -o bin/worker ./cmd/worker
	go build -o bin/starter ./cmd/starter

clean:
	rm -rf bin

run-worker:
	go run ./cmd/worker

deploy-bpmn:
	go run ./cmd/starter -deploy

deploy-all:
	go run ./cmd/starter -deploy

start-instance:
	go run ./cmd/starter -start -scenario success

start-review:
	go run ./cmd/starter -start -scenario review

approve-review:
	go run ./cmd/starter -approve

reject-review:
	go run ./cmd/starter -reject

start-decline:
	go run ./cmd/starter -start -scenario decline

start-risk-platinum:
	go run ./cmd/starter -start -scenario risk-platinum

start-risk-fraud:
	go run ./cmd/starter -start -scenario risk-fraud

start-risk-high:
	go run ./cmd/starter -start -scenario risk-high

start-risk-stock:
	go run ./cmd/starter -start -scenario risk-stock

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
