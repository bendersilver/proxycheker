# proxycheker

### Example

```go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"time"

	"github.com/bendersilver/proxycheker"
)

func errWrap(p *proxycheker.ProxyItem) {

}

func sucWrap(p *proxycheker.ProxyItem) {
	fmt.Printf("%s\n", p.Rsp.String())
}

func main() {
	runtime.GOMAXPROCS(1)
	st := proxycheker.Settings{
		CheckURL:    "https://api.ipify.org",
		NumThread:   5, // NumThread >= 2
		Success:     sucWrap, // or nill
		Error:       errWrap, // or nill
		DialTimeout: time.Second * 3,
		ConnTimeout: time.Second * 5,
	}
	if err := st.Init(); err != nil {
		panic(err)
	}
	var arr map[string]string
	// kev - host : val - port
	b, _ := ioutil.ReadFile("proxy.json")
	json.Unmarshal(b, &arr)

	for host, tp := range arr {
		st.Check(host, tp)
	}
	st.Wait()
}
```