package puredns

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
	"golang.org/x/sync/semaphore"
)

type udpConnPool struct {
	pool *pool[net.Conn]
}

func newUdpConnPool(newUpdConn func() net.Conn, idle int) (udpConnPool, error) {
	const (
		initialSize = 10
		minimumIdle = 20
	)

	if idle < minimumIdle {
		idle = minimumIdle * 2
	}
	maxCap := idle * 2

	p, err := newPool(&poolConfig[net.Conn]{
		InitialCap: initialSize,
		MaxCap:     maxCap,
		MaxIdle:    idle,
		Factory: func() (net.Conn, error) {
			return newUpdConn(), nil
		},
		Close: func(conn net.Conn) error {
			slog.Info("关闭连接")
			return conn.Close()
		},
		Ping: func(conn net.Conn) error {
			return nil
		},
		IdleTimeout: time.Second * 50,
	})
	if err != nil {
		return udpConnPool{}, err
	}
	return udpConnPool{
		pool: p,
	}, nil
}

func (u *udpConnPool) put(udpConn net.Conn) {
	_ = u.pool.Put(udpConn)
}

func (u *udpConnPool) get() net.Conn {
	conn, _ := u.pool.Get()
	return conn
}

type Client struct {
	smp           *semaphore.Weighted
	udpConnPool   udpConnPool
	globalFactory *miekg.GlobalLookupFactory
	logger        *slog.Logger
}

func NewClient(logger *slog.Logger, timeout time.Duration, parallelSize int) (*Client, error) {
	gc := zdns.GlobalConf{
		FollowCName:         true,
		IterativeResolution: true,
		//RecycleSockets:      true,
		Timeout:          timeout,
		IterationTimeout: timeout,
		Class:            dns.ClassINET,
		NameServers:      zdns.RootServers[:],
		MaxDepth:         10,
		CacheSize:        10000,
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
			return nil, errors.Join(ErrUnreachableDns, err)
		}

		gc.LocalAddrs = append(gc.LocalAddrs, conn.LocalAddr().(*net.UDPAddr).IP)
		_ = conn.Close()
	}

	glf := new(miekg.GlobalLookupFactory)
	_ = glf.Initialize(&gc)

	cli := Client{
		smp:           semaphore.NewWeighted(int64(parallelSize)),
		globalFactory: glf,
		logger:        logger,
	}

	p, err := newUdpConnPool(cli.newUdpConn, parallelSize)
	if err != nil {
		return nil, err
	}

	cli.udpConnPool = p
	return &cli, nil
}

func (c *Client) newUdpConn() net.Conn {
	const (
		maxTry = 3
	)

	var (
		err error
	)
	for i := 0; i < maxTry; i++ {
		var conn net.Conn
		conn, err = net.ListenUDP("udp", &net.UDPAddr{
			IP: c.globalFactory.RandomLocalAddr(),
		})

		if err == nil {
			return conn
		}

		c.logger.Warn("监听upd端口失败", slog.Int("try", i), slog.String("err", err.Error()))
		time.Sleep(time.Second)
	}

	panic(err)
}

type LookupResult struct {
	Name   string
	Answer []RR
}

type RR struct {
	Name  string
	Type  dns.Type
	TTL   uint32
	Value string
}

type clientLogger struct{}

func SetLogger(ctx context.Context, handler *slog.Logger) context.Context {
	return context.WithValue(ctx, clientLogger{}, handler)
}

func (c *Client) getLockupLogger(ctx context.Context) *slog.Logger {
	v := ctx.Value(clientLogger{})
	if v == nil {
		return c.logger
	}

	return v.(*slog.Logger)
}

func (c *Client) Lookups(ctx context.Context, resolveTy dns.Type, arr []string) []LookupResult {
	var (
		wg        sync.WaitGroup
		lockups   []LookupResult
		lockupsMu sync.Mutex
		logger    = c.getLockupLogger(ctx)
	)

	arr = lo.Uniq(arr)
	for _, s := range arr {
		err := c.smp.Acquire(ctx, 1)
		if err != nil {
			break
		}

		wg.Add(1)
		name := s
		go func() {
			defer c.smp.Release(1)
			defer wg.Done()

			r := miekg.RoutineLookupFactory{
				Factory:  c.globalFactory,
				DNSType:  uint16(resolveTy),
				ThreadID: 0,
			}

			r.Initialize(c.globalFactory.GlobalConf)
			// 设置conn，复用udp 监听连接
			r.Conn = new(dns.Conn)
			r.Conn.Conn = c.udpConnPool.get()
			udpConn := r.Conn.Conn

			defer func() {
				_ = udpConn.SetDeadline(time.Time{})
				r.Conn.Conn = nil

				c.udpConnPool.put(udpConn)
			}()

			logger := logger.With(slog.String("name", name))
			lookup, _ := r.MakeLookup()

			res, _, status, err := lookup.DoLookup(name, "")
			if err != nil {
				logger.Warn("dns解析失败", slog.String("err", err.Error()))
				return
			}

			if status == zdns.STATUS_ERROR {
				logger.Warn("错误的dns状态", slog.String("status", string(status)))
				return
			}

			result, ok := res.(miekg.Result)
			if !ok {
				logger.Error("解析响应结果类型出错")
				return
			}

			resultAnswer := lo.FilterMap(result.Answers, func(item any, index int) (miekg.Answer, bool) {
				answer, ok := item.(miekg.Answer)

				return answer, ok
			})
			if len(resultAnswer) == 0 {
				return
			}

			lockupsMu.Lock()
			defer lockupsMu.Unlock()

			lockups = append(lockups, LookupResult{
				Name: name,
				Answer: lo.Uniq(lo.Map(resultAnswer, func(item miekg.Answer, index int) RR {
					return RR{
						Name:  item.Name,
						TTL:   item.Ttl,
						Type:  dns.Type(dns.StringToType[item.Type]),
						Value: item.Answer,
					}
				})),
			})
		}()
	}

	wg.Wait()

	return lockups
}
