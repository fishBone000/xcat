package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/fishBone000/xcat/log"
)

const (
	ModeServer = "server"
	ModeClient = "client"
)

// Cmd line arguments
var (
	Mode                  string
	Host                  string
	Port                  int
	Usr                   string
	Pwd                   string
	LAddr                 string
	LHost                 string
	DataLinkListenTimeout uint // TODO int might be better for these timeouts? What if they overflow?
	CtrlLinkTimeout       uint
	UDPTimeout            uint
	Version               bool
	LogLevel              int
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
	flag.UintVar(&DataLinkListenTimeout, "t", 15, "timeout (sec) for listening incoming data link, effective on server side only")
	flag.UintVar(&CtrlLinkTimeout, "T", 5, "timeout (sec) for establishing control link an dport query, effective on client side only")
	flag.UintVar(&UDPTimeout, "u", 180, "timeout (sec) for UDP relaying, effective on client side only")
	flag.BoolVar(&Version, "v", false, "print version number")
	flag.IntVar(&LogLevel, "d", 2, "log level, 0: err, 1: warn, 2: info, 3: dbg")
}

func init() {
	specifyFlags()

	flag.Parse()

	checkFlags()
}

func checkFlags() {
	if Version {
		fmt.Println(version)
		os.Exit(0)
	}

	if Mode != ModeServer && Mode != ModeClient {
		fmt.Printf("Unknown mode %s", Mode)
		os.Exit(1)
	}

	if Port < 0x00 || Port > 0xFFFF {
		fmt.Printf("Invalid port %d. \n", Port)
		os.Exit(1)
	}
	Addr = net.JoinHostPort(Host, strconv.Itoa(Port))

	var err error
	LHost, _, err = net.SplitHostPort(LAddr)
	if err != nil {
		fmt.Printf("Invalid listening address %s: %s. \n", LAddr, err.Error())
		os.Exit(1)
	}

	log.Level = LogLevel
}
