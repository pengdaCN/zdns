package main

import (
	"bufio"
	"bytes"
	"context"
	"github.com/samber/lo"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/puredns"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
)

func main() {
	arr := readStringsFromFile(os.Args[1])

	client, err := puredns.NewClient(slog.Default(), time.Second*5, 1000)
	if err != nil {
		panic(err)
	}

	lookups := client.Lookups(context.Background(), dns.Type(dns.TypeA), arr)
	names := lo.Map(lookups, func(item puredns.LookupResult, index int) string {
		return item.Name
	})

	lo.Uniq(names)
	slices.Sort(names)

	err = os.WriteFile(os.Args[2], []byte(strings.Join(names, "\n")), 0o644)
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
