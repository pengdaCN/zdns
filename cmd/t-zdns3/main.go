package main

import (
	"bufio"
	"bytes"
	"github.com/samber/lo"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	_ "github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"
)

func main() {
	gc := zdns.GlobalConf{
		Module:              "A",
		FollowCName:         true,
		IterativeResolution: true,
		Retries:             3,
		RecycleSockets:      true,
		Timeout:             time.Second * 15,
		IterationTimeout:    time.Second * 4,
		Class:               dns.ClassINET,
		NameServers:         zdns.RootServers[:],
		MaxDepth:            10,
		CacheSize:           1000,
		//UDPOnly:             true,
		//InputFilePath:   os.Args[1],
		//OutputFilePath:  os.Args[2],
		Verbosity: 1,
		//ResultVerbosity: "normal",
		Threads: 1000,
	}

	in := make(chan string)
	out := make(chan any)

	go zdns.Run2(gc, in, out)

	arr := readStringsFromFile(os.Args[1])
	arr = lo.Uniq(arr)
	go func() {
		defer close(in)
		for _, s := range arr {
			in <- s
		}
	}()

	var rr []string
	for r := range out {
		zdnsResult := r.(zdns.Result)
		res := zdnsResult.Data
		if r == nil {
			continue
		}

		result, ok := res.(miekg.Result)
		if !ok {
			slog.Error("解析响应结果类型出错")
			continue
		}

		resultAnswer := lo.FilterMap(result.Answers, func(item any, index int) (miekg.Answer, bool) {
			answer, ok := item.(miekg.Answer)
			if !ok {
				slog.Warn("未知的类型", slog.String("类型", reflect.TypeOf(item).String()))
			}

			return answer, ok
		})
		if len(resultAnswer) == 0 {
			continue
		}

		rr = append(rr, zdnsResult.Name)
	}

	err := os.WriteFile(os.Args[2], []byte(strings.Join(rr, "\n")), 0o644)
	if err != nil {
		panic(err)
	}
}

func readStringsFromFile(path string) []string {
	bs, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(bs))
	var r []string
	for scanner.Scan() {
		r = append(r, scanner.Text())
	}

	return r
}
