package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

const (
	ModeServer = "server"
	ModeClient = "client"
)

// Cmd line arguments
var (
	Mode  string
	Host  string
	Port  int
	Usr   string
	Pwd   string
	LAddr string
  LHost string
)

// Variables after parsing
var (
  Addr string // Combination of Host and Port
)

func specifyFlags() {
	flag.StringVar(&Mode, "m", "", "run mode, can be server or client, cannot be empty")
	flag.StringVar(&Host, "h", "", "host name")
	flag.IntVar(&Port, "p", 0, "port")
	flag.StringVar(&Usr, "U", "", "username for authentication")
	flag.StringVar(&Pwd, "P", "", "password for authentication")
	flag.StringVar(&LAddr, "l", ":1080", "listening address")
}

func init() {
	specifyFlags()

	flag.Parse()

	checkFlags()
}

func checkFlags() {
	if Mode != ModeServer && Mode != ModeClient {
		os.Exit(1)
	}

	if Port < 0x00 || Port > 0xFFFF {
		fmt.Printf("Invalid port %d", Port)
		os.Exit(1)
	}
  Addr = net.JoinHostPort(Host, strconv.Itoa(Port))

  var err error
  LHost, _, err = net.SplitHostPort(LAddr)
  if err != nil {
    fmt.Printf("Invalid listening address %s: %s", LAddr, err.Error())
		os.Exit(1)
  }
}
