package puredns

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
)

func Lookup(dnsTy dns.Type, v string) error {
	lookupFactory := zdns.GetLookup(dnsTy.String())
	if lookupFactory == nil {
		return ErrUnsupportedDnsType
	}

	gc := zdns.GlobalConf{
		FollowCName:         true,
		IterativeResolution: true,
		Timeout:             time.Second * 6,
		IterationTimeout:    time.Second * 6,
		Class:               dns.ClassINET,
		NameServers:         zdns.RootServers[:],
		RecycleSockets:      true,
		MaxDepth:            10,
		CacheSize:           10000,
		UDPOnly:             true,
	}

	// 发现本地一个可以发送数据的地址
	// TODO 后续可以考虑使用 0.0.0.0 ip 替代
	// Find local address for use in unbound UDP sockets
	{
		conn, err := net.Dial("udp", "8.8.8.8:53")
		if err != nil {
			return errors.Join(ErrUnreachableDns, err)
		}

		gc.LocalAddrs = append(gc.LocalAddrs, conn.LocalAddr().(*net.UDPAddr).IP)
		_ = conn.Close()
	}

	if err := lookupFactory.Initialize(&gc); err != nil {
		return err
	}

	routineFactory, err := lookupFactory.MakeRoutineFactory(1)
	if err != nil {
		return err
	}

	lookup, err := routineFactory.MakeLookup()
	if err != nil {
		return err
	}

	data, _, status, err := lookup.DoLookup(v, "")
	if err != nil {
		return err
	}

	if status != zdns.STATUS_NOERROR {
		return errors.Join(ErrInvalidDnsStatus, errors.New(string(status)))
	}

	answer, ok := data.(miekg.Result)
	if !ok {
		return ErrInvalidData
	}

	fmt.Println(answer)

	if err := lookupFactory.Finalize(); err != nil {
		return err
	}

	return nil
}
