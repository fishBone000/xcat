package main

var version = "undefined"

const udpIoRetries = 4

func main() {
	switch Mode {
	case ModeServer:
		runServer()
	case ModeClient:
		runClient()
	}
}
