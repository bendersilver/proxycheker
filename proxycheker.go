package proxycheker

import (
	"fmt"
	"net"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/bendersilver/proxyreq"
	"golang.org/x/net/proxy"
)

// ProxyItem -
type ProxyItem struct {
	Host string
	Type string
	Err  error
	Time time.Duration
}

// newProxyItem -
func newProxyItem(host, typ string) *ProxyItem {
	return &ProxyItem{
		Host: host,
		Type: typ,
	}
}

// Settings -
type Settings struct {
	wg           sync.WaitGroup
	CheckURL     string
	NumThread    int
	Success      func(*ProxyItem)
	Error        func(*ProxyItem)
	DialTimeout  time.Duration
	ConnTimeout  time.Duration
	DisableHTTPS bool

	chanProxy chan *ProxyItem
}

// Wait -
func (s *Settings) Wait() {
	s.wg.Wait()
}

// Init -
func (s *Settings) worker() {
	var dialer proxy.Dialer
	var conn net.Conn
	for {
		select {
		case p := <-s.chanProxy:
			s.wg.Add(1)
			now := time.Now()
			dialer, p.Err = proxyreq.Dialer(p.Host, p.Type)
			if p.Err != nil {
				if s.Error != nil {
					s.Error(p)
				}
			} else {
				conn, p.Err = dialer.Dial("tcp", s.CheckURL)
				if p.Err != nil {
					if s.Error != nil {
						s.Error(p)
					}
				} else {
					conn.Close()
					p.Time = time.Now().Sub(now)
					if s.Success != nil {
						s.Success(p)
					}
				}
			}
			s.wg.Done()
		}
	}
}

// Init -
func (s *Settings) Init() error {
	runtime.GOMAXPROCS(1)

	_, err := url.Parse(s.CheckURL)
	if err != nil {
		return err
	}
	if s.NumThread < 2 {
		return fmt.Errorf("number of threads less than 2")
	}
	if s.DialTimeout < time.Millisecond {
		return fmt.Errorf("DialTimeout less than a millisecond")
	}
	proxyreq.SetDialTimeout(s.DialTimeout)
	if s.ConnTimeout < time.Millisecond {
		return fmt.Errorf("ConnTimeout less than a millisecond")
	}
	proxyreq.SetConnTimeout(s.ConnTimeout)
	for ix := 0; ix < s.NumThread; ix++ {
		go s.worker()
	}
	s.chanProxy = make(chan *ProxyItem, 1)
	return nil
}

var typs = map[string]bool{
	"http":   true,
	"https":  true,
	"socks5": true,
}

// Check -
func (s *Settings) Check(host, tp string) {
	if ok := typs[tp]; !ok {
		for t := range typs {
			if s.DisableHTTPS && t == "https" {
				continue
			}
			s.chanProxy <- newProxyItem(host, t)
		}
	} else {
		s.chanProxy <- newProxyItem(host, tp)
	}
}
