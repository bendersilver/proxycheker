package proxycheker

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/bendersilver/proxyreq"
	"github.com/imroc/req"
)

// proxyItem -
type proxyItem struct {
	Host string
	Type string
}

// newProxyItem -
func newProxyItem(host, typ string) *proxyItem {
	return &proxyItem{
		Host: host,
		Type: typ,
	}
}

// Settings -
type Settings struct {
	sync.Mutex
	CheckURL    string
	NumThread   int
	Success     func(string, string, *req.Resp)
	Error       func(string, string, error)
	DialTimeout time.Duration
	ConnTimeout time.Duration

	chanProxy chan *proxyItem
	counter   int
	done      chan bool
}

// incr -
func (s *Settings) incr() {
	s.Lock()
	defer s.Unlock()
	s.counter++
}

// reduce -
func (s *Settings) reduce() {
	s.Lock()
	defer s.Unlock()
	s.counter--
	if s.counter == 0 {
		close(s.done)
	}
}

// Done -
func (s *Settings) Done() {
	<-s.done
}

// Init -
func (s *Settings) worker() {
	for {
		select {
		case p := <-s.chanProxy:

			s.incr()
			if r, err := proxyreq.New(p.Host, p.Type); err != nil {
				if s.Error != nil {
					s.Error(p.Host, p.Type, err)
				}
			} else {
				ctx, cancel := context.WithCancel(context.TODO())
				time.AfterFunc(s.ConnTimeout, func() {
					cancel()
				})
				if rsp, err := r.Get(s.CheckURL, ctx); err != nil {
					if s.Error != nil {
						s.Error(p.Host, p.Type, err)
					}
				} else {
					if s.Success != nil {
						s.Success(p.Host, p.Type, rsp)
					}
				}
				cancel()
				s.reduce()
			}
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
	if s.NumThread < 1 {
		return fmt.Errorf("number of threads less than 1")
	}
	if s.DialTimeout < time.Millisecond {
		return fmt.Errorf("DialTimeout less than a millisecond")
	}
	proxyreq.DialTimeout = s.DialTimeout
	if s.ConnTimeout < time.Millisecond {
		return fmt.Errorf("ConnTimeout less than a millisecond")
	}
	proxyreq.ClientTimeout = s.ConnTimeout
	s.done = make(chan bool)
	for ix := 0; ix < s.NumThread; ix++ {
		go s.worker()
	}
	s.chanProxy = make(chan *proxyItem, s.NumThread)
	return nil
}

var typs = map[string]bool{
	"http":   true,
	"https":  true,
	"socks5": true,
}

// Check -
func (s *Settings) Check(host, tp string) {
	if len(host) >= 7 {
		if ok := typs[tp]; !ok {
			for t := range typs {
				s.chanProxy <- newProxyItem(host, t)
			}
		} else {
			s.chanProxy <- newProxyItem(host, tp)
		}
	}
}
