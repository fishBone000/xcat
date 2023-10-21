package main

var version = "undefined"

func main() {
	switch Mode {
	case ModeServer:
		runServer()
	case ModeClient:
		runClient()
	}
}
