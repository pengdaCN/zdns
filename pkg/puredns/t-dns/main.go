package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/puredns"
)

func main() {
	ty := os.Args[1]
	domainsPath := os.Args[2]
	parallelSize, err := strconv.Atoi(os.Args[3])
	if err != nil {
		panic(err)
	}
	resultPath := os.Args[4]

	cli, err := puredns.NewClient(slog.Default(), time.Second*3, parallelSize)
	if err != nil {
		panic(err)
	}

	domains := readDomainsFromFile(domainsPath)

	now := time.Now()
	results := cli.Lookups(context.Background(), dns.Type(dns.StringToType[ty]), domains)
	slog.Info("执行耗时",
		slog.Duration("cost", time.Since(now)),
		slog.Int("域名", len(domains)),
		slog.Int("结果数量", len(results)),
	)
	resultFile, err := os.OpenFile(resultPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		panic(err)
	}

	for _, rr := range results {
		if _, err := resultFile.WriteString(fmt.Sprintln("name =>", rr.Name)); err != nil {
			panic(err)
		}
		if _, err := resultFile.WriteString(fmt.Sprintln("value =>", strings.Join(lo.Map(rr.Answer, func(item puredns.RR, index int) string {
			return item.Value
		}), " "))); err != nil {
			panic(err)
		}
	}
}

func readDomainsFromFile(domainsFilePath string) []string {
	bs, err := os.ReadFile(domainsFilePath)
	if err != nil {
		panic(err)
	}

	return lo.FilterMap(bytes.Split(bs, []byte("\n")), func(item []byte, index int) (string, bool) {
		if string(item) == "" {
			return "", false
		}

		return strings.TrimSpace(string(item)), true
	})
}
