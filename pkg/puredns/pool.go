package puredns

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	//ErrMaxActiveConnReached 连接池超限
	ErrMaxActiveConnReached = errors.New("MaxActiveConnReached")
	//ErrClosed 连接池已经关闭Error
	ErrClosed = errors.New("pool is closed")
)

// poolConfig 连接池相关配置
type poolConfig[T any] struct {
	//连接池中拥有的最小连接数
	InitialCap int
	//最大并发存活连接数
	MaxCap int
	//最大空闲连接
	MaxIdle int
	//生成连接的方法
	Factory func() (T, error)
	//关闭连接的方法
	Close func(T) error
	//检查连接是否有效的方法
	Ping func(T) error
	//连接最大空闲时间，超过该事件则将失效
	IdleTimeout time.Duration
}

type connReq[T any] struct {
	idleConn *idleConn[T]
}

// pool 存放连接信息
type pool[T any] struct {
	mu                       sync.RWMutex
	conns                    chan *idleConn[T]
	factory                  func() (T, error)
	close                    func(T) error
	ping                     func(T) error
	idleTimeout, waitTimeOut time.Duration
	maxActive                int
	openingConns             int
	connReqs                 []chan connReq[T]
}

type idleConn[T any] struct {
	conn T
	t    time.Time
}

func newPool[T any](poolConfig *poolConfig[T]) (*pool[T], error) {
	if !(poolConfig.InitialCap <= poolConfig.MaxIdle && poolConfig.MaxCap >= poolConfig.MaxIdle && poolConfig.InitialCap >= 0) {
		return nil, errors.New("invalid capacity settings")
	}
	if poolConfig.Factory == nil {
		return nil, errors.New("invalid factory func settings")
	}
	if poolConfig.Close == nil {
		return nil, errors.New("invalid close func settings")
	}

	c := &pool[T]{
		conns:        make(chan *idleConn[T], poolConfig.MaxIdle),
		factory:      poolConfig.Factory,
		close:        poolConfig.Close,
		idleTimeout:  poolConfig.IdleTimeout,
		maxActive:    poolConfig.MaxCap,
		openingConns: poolConfig.InitialCap,
	}

	if poolConfig.Ping != nil {
		c.ping = poolConfig.Ping
	}

	for i := 0; i < poolConfig.InitialCap; i++ {
		conn, err := c.factory()
		if err != nil {
			c.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- &idleConn[T]{conn: conn, t: time.Now()}
	}

	return c, nil
}

// getConns 获取所有连接
func (c *pool[T]) getConns() chan *idleConn[T] {
	c.mu.Lock()
	conns := c.conns
	c.mu.Unlock()
	return conns
}

// Get 从pool中取一个连接
func (c *pool[T]) Get() (conn T, err error) {
	conns := c.getConns()
	if conns == nil {

		return conn, ErrClosed
	}
	for {
		select {
		case wrapConn := <-conns:
			if wrapConn == nil {
				return conn, ErrClosed
			}
			//判断是否超时，超时则丢弃
			if timeout := c.idleTimeout; timeout > 0 {
				if wrapConn.t.Add(timeout).Before(time.Now()) {
					//丢弃并关闭该连接
					_ = c.Close(wrapConn.conn)
					continue
				}
			}
			//判断是否失效，失效则丢弃，如果用户没有设定 ping 方法，就不检查
			if c.ping != nil {
				if err := c.Ping(wrapConn.conn); err != nil {
					_ = c.Close(wrapConn.conn)
					continue
				}
			}
			return wrapConn.conn, nil
		default:
			c.mu.Lock()
			if c.openingConns >= c.maxActive {
				req := make(chan connReq[T], 1)
				c.connReqs = append(c.connReqs, req)
				c.mu.Unlock()
				ret, ok := <-req
				if !ok {
					return conn, ErrMaxActiveConnReached
				}
				if timeout := c.idleTimeout; timeout > 0 {
					if ret.idleConn.t.Add(timeout).Before(time.Now()) {
						//丢弃并关闭该连接
						_ = c.Close(ret.idleConn.conn)
						continue
					}
				}
				return ret.idleConn.conn, nil
			}
			if c.factory == nil {
				c.mu.Unlock()
				return conn, ErrClosed
			}
			conn, err := c.factory()
			if err != nil {
				c.mu.Unlock()
				return conn, err
			}
			c.openingConns++
			c.mu.Unlock()
			return conn, nil
		}
	}
}

// Put 将连接放回pool中
func (c *pool[T]) Put(conn T) error {
	c.mu.Lock()
	if c.conns == nil {
		c.mu.Unlock()
		return c.Close(conn)
	}

	if l := len(c.connReqs); l > 0 {
		req := c.connReqs[0]
		copy(c.connReqs, c.connReqs[1:])
		c.connReqs = c.connReqs[:l-1]
		req <- connReq[T]{
			idleConn: &idleConn[T]{conn: conn, t: time.Now()},
		}
		c.mu.Unlock()
		return nil
	} else {
		select {
		case c.conns <- &idleConn[T]{conn: conn, t: time.Now()}:
			c.mu.Unlock()
			return nil
		default:
			c.mu.Unlock()
			//连接池已满，直接关闭该连接
			return c.Close(conn)
		}
	}
}

// Close 关闭单条连接
func (c *pool[T]) Close(conn T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.close == nil {
		return nil
	}
	c.openingConns--
	return c.close(conn)
}

// Ping 检查单条连接是否有效
func (c *pool[T]) Ping(conn T) error {
	return c.ping(conn)
}

// Release 释放连接池中所有连接
func (c *pool[T]) Release() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.factory = nil
	c.ping = nil
	closeFun := c.close
	c.close = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)
	for wrapConn := range conns {
		_ = closeFun(wrapConn.conn)
	}
}

// Len 连接池中已有的连接
func (c *pool[T]) Len() int {
	return len(c.getConns())
}
