package main

func main() {
  switch Mode {
  case ModeServer:
    runServer()
  case ModeClient:
    runClient()
  }
}
