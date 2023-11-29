package main

import (
	"bufio"
	"bytes"
	"context"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
	"golang.org/x/sync/semaphore"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

func main() {
	smp := semaphore.NewWeighted(1000)
	arr := readStringsFromFile(os.Args[1])
	gc := zdns.GlobalConf{
		FollowCName:         true,
		IterativeResolution: true,
		RecycleSockets:      true,
		Timeout:             4 * time.Second,
		IterationTimeout:    15 * time.Second,
		Class:               dns.ClassINET,
		NameServers:         zdns.RootServers[:],
		MaxDepth:            10,
		CacheSize:           10000,
		//UDPOnly:          true,
		//LocalAddrs: []net.IP{
		//	net.ParseIP("0.0.0.0"),
		//},
	}

	// 发现本地一个可以发送数据的地址
	// TODO 后续可以考虑使用 0.0.0.0 ip 替代
	// Find local address for use in unbound UDP sockets
	{
		conn, err := net.Dial("udp", "8.8.8.8:53")
		if err != nil {
			panic(err)
		}

		gc.LocalAddrs = append(gc.LocalAddrs, conn.LocalAddr().(*net.UDPAddr).IP)
		_ = conn.Close()
	}

	glf := new(miekg.GlobalLookupFactory)
	glf.SetDNSType(dns.TypeA)
	if err := glf.Initialize(&gc); err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, s := range arr {
		wg.Add(1)
		_ = smp.Acquire(context.Background(), 1)

		name := s
		go func() {
			defer smp.Release(1)
			defer wg.Done()

			lk, _ := glf.MakeRoutineFactory(0)
			l, err := lk.MakeLookup()
			if err != nil {
				panic(err)
			}

			lookup, _, _, err := l.DoLookup(name, "")
			if err != nil {
				slog.Error("解析识别", slog.String("域名", name), slog.String("err", err.Error()))
				return
			}

			r := lookup.(miekg.Result)

			if len(r.Answers) == 0 {
				return
			}

			slog.Info("解析成功", slog.String("域名", name))
		}()
	}

	wg.Wait()
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
