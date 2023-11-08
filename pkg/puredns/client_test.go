package puredns

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/zmap/dns"
)

func TestClient_Lookups(t *testing.T) {
	client, err := NewClient(slog.Default(), time.Second*3, 1000)
	if err != nil {
		t.Fatal(err)
	}

	bs, err := os.ReadFile("/home/u001/d1.txt")
	if err != nil {
		t.Fatal(err)
	}

	domains := strings.Split(string(bs), "\n")
	domains = lo.Filter(domains, func(item string, index int) bool {
		return item != ""
	})

	rrs := client.Lookups(context.Background(), dns.Type(dns.TypeCNAME), domains)
	t.Log("domains", len(domains))
	t.Log("rrs", len(rrs))

	for _, rr := range rrs {
		t.Log("name =>", rr.Name)
		t.Log("value =>", strings.Join(lo.Map(rr.Answer, func(item RR, index int) string {
			return item.Value
		}), " "))
	}
}
