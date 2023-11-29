package main

import (
	"os"
	"time"

	"github.com/spf13/pflag"
	"github.com/zmap/dns"
	_ "github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
)

func main() {
	gc := zdns.GlobalConf{
		Module:              "A",
		FollowCName:         true,
		IterativeResolution: true,
		Retries:             3,
		//RecycleSockets:      true,
		Timeout:          time.Second * 4 * 2,
		IterationTimeout: time.Second * 4,
		Class:            dns.ClassINET,
		NameServers:      zdns.RootServers[:],
		MaxDepth:         10,
		CacheSize:        1000,
		//UDPOnly:             true,
		InputFilePath:   os.Args[1],
		OutputFilePath:  os.Args[2],
		Verbosity:       1,
		ResultVerbosity: "normal",
		Threads:         1000,
	}

	pflag.NewFlagSet("", pflag.ExitOnError)
	timeout := 15
	iterationTimeout := 4
	classStr := "INET"
	f := pflag.NewFlagSet("", pflag.ExitOnError)
	f.String("blacklist-file", "", "")
	zdns.Run(gc, f, &timeout, &iterationTimeout,
		&classStr, new(string),
		new(string), new(string),
		new(string), new(bool),
		new(string), new(bool),
	)
}
