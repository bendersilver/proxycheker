package proxycheker

import (
	"net"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/bendersilver/proxyreq"
	"github.com/dustin/go-broadcast"
	"github.com/imroc/req"
	"golang.org/x/net/proxy"
)

// ProxyItem -
type ProxyItem struct {
	Host string
	Type string
	Err  error
	Resp *req.Resp
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
	br        broadcast.Broadcaster
}

// Wait -
func (s *Settings) Wait() {
	s.wg.Wait()
	s.br.Submit(nil)
	s.br.Close()
	runtime.GC()
}

// Init -
func (s *Settings) worker() {
	var dialer proxy.Dialer
	var conn net.Conn
	rq := proxyreq.NewEmpty()

	ch := make(chan interface{})
	s.br.Register(ch)
	for {
		select {
		case <-ch:
			dialer = nil
			conn = nil
			s.br.Unregister(ch)
			close(ch)
			return
		case p := <-s.chanProxy:
			s.wg.Add(1)
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

					rq.SetTransport(p.Host, p.Type)
					var ur url.URL
					hs, prt, _ := net.SplitHostPort(s.CheckURL)
					ur.Host = hs
					ur.Scheme = "http"
					switch prt {
					case "443":
						ur.Scheme = "https"
					default:
						ur.Host = s.CheckURL
					}
					p.Resp, p.Err = rq.Get(ur.String())
					if p.Err != nil {
						if s.Error != nil {
							s.Error(p)
						}
					} else {
						if s.Success != nil {
							s.Success(p)
						}
					}
				}
			}
			s.wg.Done()
		}
	}
}

// Init -
func (s *Settings) Init() error {
	_, err := url.Parse(s.CheckURL)
	if err != nil {
		return err
	}
	s.chanProxy = make(chan *ProxyItem)
	if s.NumThread < 1 {
		s.NumThread = 1
	}
	s.br = broadcast.NewBroadcaster(s.NumThread)
	for ix := 0; ix < s.NumThread; ix++ {
		go s.worker()
	}
	proxyreq.SetDialTimeout(s.DialTimeout)
	proxyreq.SetConnTimeout(s.ConnTimeout)
	return nil
}

var typs = map[string]bool{
	"socks5": true,
	"https":  true,
	"http":   true,
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
