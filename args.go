package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	ModeServer = "server"
	ModeClient = "client"
)

var (
	Mode    string
	Host    string
	Port    int
	Usr     string
	Pwd     string
	LAddr   string
)

func specifyFlags() {
	flag.StringVar(&Mode, "m", "", "run mode, can be server or client, cannot be empty")
	flag.StringVar(&Host, "h", "", "host name")
	flag.IntVar(&Port, "p", 0, "port")
	flag.StringVar(&Usr, "U", "", "username for authentication")
	flag.StringVar(&Pwd, "P", "", "password for authentication")
  flag.StringVar(&LAddr, "l", ":1080", "listening address in client mode")
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
		fmt.Printf("invalid port %d", Port)
		os.Exit(1)
	}
}
