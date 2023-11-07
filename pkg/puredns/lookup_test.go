package puredns

import (
	"github.com/zmap/dns"
	"testing"
)

func TestLookup(t *testing.T) {
	err := Lookup(dns.Type(dns.TypeA), `www.baidu.com`)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ok")
}

func TestTypeAsset(t *testing.T) {
	var x any

	v, ok := x.(int)

	t.Log(v, ok)
}
