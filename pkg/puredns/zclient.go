package puredns

import (
	"context"
	"log/slog"
	"net"
	"reflect"
	"time"

	"github.com/samber/lo"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
)

type ZClient struct {
	gc     zdns.GlobalConf
	logger *slog.Logger
}

func NewZClient(logger *slog.Logger, timeout time.Duration, iterationTimeout time.Duration, parallelSize int) (*ZClient, error) {
	gc := zdns.GlobalConf{
		FollowCName:         true,
		IterativeResolution: true,
		Retries:             3,
		RecycleSockets:      true,
		Timeout:             timeout,
		IterationTimeout:    iterationTimeout,
		Class:               dns.ClassINET,
		NameServers:         zdns.RootServers[:],
		MaxDepth:            10,
		CacheSize:           1000,
		Threads:             parallelSize,
	}

	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	gc.LocalAddrs = append(gc.LocalAddrs, conn.LocalAddr().(*net.UDPAddr).IP)
	_ = conn.Close()

	return &ZClient{
		gc:     gc,
		logger: logger,
	}, nil
}

func (c *ZClient) getLockupLogger(ctx context.Context) *slog.Logger {
	v := ctx.Value(clientLogger{})
	if v == nil {
		return c.logger
	}

	return v.(*slog.Logger)
}

func (c *ZClient) Lookups(ctx context.Context, resolveTy dns.Type, arr []string) []LookupResult {
	in := make(chan string)
	out := make(chan any)

	arr = lo.Uniq(arr)
	go func() {
		defer close(in)
		for _, s := range arr {
			select {
			case <-ctx.Done():
				return
			case in <- s:
			}
		}
	}()

	execGc := c.gc
	execGc.Module = resolveTy.String()

	var logger = c.getLockupLogger(ctx)
	go func() {
		if err := zdns.Run2(execGc, in, out); err != nil {
			logger.Error("执行dns查询错误", slog.String("err", err.Error()))
			return
		}
	}()

	var rr []LookupResult
	for r := range out {
		zdnsResult := r.(zdns.Result)
		res := zdnsResult.Data
		if r == nil {
			continue
		}

		result, ok := res.(miekg.Result)
		if !ok {
			logger.Error("解析响应结果类型出错")
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

		rr = append(rr, LookupResult{
			Name: zdnsResult.Name,
			Answer: lo.Uniq(lo.Map(resultAnswer, func(item miekg.Answer, index int) RR {
				return RR{
					Name:  item.Name,
					TTL:   item.Ttl,
					Type:  dns.Type(dns.StringToType[item.Type]),
					Value: item.Answer,
				}
			})),
		})
	}

	return rr

}
