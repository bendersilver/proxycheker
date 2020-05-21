package proxycheker

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/bendersilver/proxyreq"
	"github.com/imroc/req"
)

// ProxyItem -
type ProxyItem struct {
	Host string
	Type string
	Err  error
	Rsp  *req.Resp
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
	mx          sync.Mutex
	CheckURL    string
	NumThread   int
	Success     func(*ProxyItem)
	Error       func(*ProxyItem)
	DialTimeout time.Duration
	ConnTimeout time.Duration

	chanProxy chan *ProxyItem
	counter   int
	done      chan bool
}

// incr -
func (s *Settings) incr() {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.counter++
}

// reduce -
func (s *Settings) reduce() {
	s.mx.Lock()
	defer s.mx.Unlock()
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
	rq := proxyreq.NewEmpty()
	ur := &url.URL{}
	for {
		select {
		case p := <-s.chanProxy:
			s.incr()
			ur.Host = p.Host
			ur.Scheme = p.Type
			if p.Err = rq.SetTransport(ur); p.Err != nil {
				if s.Error != nil {
					s.Error(p)
				}
			} else {
				if p.Rsp, p.Err = rq.Get(s.CheckURL); p.Err != nil {
					if s.Error != nil {
						s.Error(p)
					}
				} else {
					if s.Success != nil {
						s.Success(p)
					}
				}
			}
			s.reduce()
		}
	}
}

// Init -
func (s *Settings) Init() error {
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
	s.chanProxy = make(chan *ProxyItem)
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
			s.chanProxy <- newProxyItem(host, t)
		}
	} else {
		s.chanProxy <- newProxyItem(host, tp)
	}
}
