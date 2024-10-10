# Chat DHT

This is a simple implementation of a Distributed Hash Table (DHT) for a chat application.

## How to run

1. Clone the repository
2. Run the following command to install the dependencies:
```bash
go mod tidy
```
3. Run the following command to start the server:
```bash
go run chat_dht.go host:port 
```

## How to chat with other users

### Join a chat room
To join a chat room, you need to know the address of a peer that is already in the chat room. You can get this address by running the following command:
```bash
go run chat_dht.go host:port [host:port]
```

## Example
### Terminal 1
```bash
go run chat_dht.go 127.0.0.1:8001
```
### Terminal 2
```bash
go run chat_dht.go 127.0.0.1:8002 127.0.0.1:8001
```
