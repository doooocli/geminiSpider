package main

import (
	"context"
	"fmt"
	"github.com/google/generative-ai-go/genai"
	"github.com/spf13/viper"
	"google.golang.org/api/option"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type GenaiClientPool struct {
	keysProxy     sync.Map
	httpProxyPool []string
	keysCount     atomic.Int32
}

func (g *GenaiClientPool) getProxyURL(key string) string {
	if proxyUrl, ok := g.keysProxy.Load(key); ok {
		return proxyUrl.(string)
	}

	pLen := len(g.httpProxyPool)
	if pLen == 0 {
		return ""
	}
	keysCount := g.keysCount.Load()
	g.keysCount.Add(1)
	if int32(pLen) > keysCount {
		p := g.httpProxyPool[keysCount]
		g.keysProxy.Store(key, p)
		return p
	}
	// 随机一个
	p := g.httpProxyPool[rand.Intn(pLen)]
	g.keysProxy.Store(key, p)
	return p
}

func getUserAgent() string {
	n := rand.Intn(len(UserAgents))
	if n == 0 {
		return UserAgents[0]
	} else {
		return UserAgents[n-1]
	}
}

func NewGenClient(ctx context.Context, key, proxyUrl string) (*genai.Client, error) {
	c := NewHttpClient(key, proxyUrl)
	return genai.NewClient(ctx,
		option.WithUserAgent(getUserAgent()),
		option.WithHTTPClient(c),
		option.WithAPIKey(key), // 解决 google.golang.org/api/client 包的 hasAuthOption 函数 无法识别 option.withHTTPClient
	)
}

func NewHttpClient(key, proxyUrl string) *http.Client {
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: createTransport(proxyUrl),
		//Transport: &ProxyRoundTripper{
		//	APIKey:   key,
		//	ProxyURL: proxyUrl,
		//},
	}
}

func createTransport(proxyStr string) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true // 关闭连接复用

	proxyConf := strings.Split(proxyStr, ":")
	if len(proxyConf) < 2 {
		return transport
	}
	var proxyUrl string
	var urlProxy *url.URL
	urlI := url.URL{}

	if viper.GetString("app.proxyType") == "http" {
		proxyUrl = fmt.Sprintf("%s:%s", proxyConf[0], proxyConf[1])
		if !strings.Contains(proxyUrl, "http") {
			proxyUrl = fmt.Sprintf("http://%s", proxyUrl)
		}
		urlProxy, _ = urlI.Parse(proxyUrl)
		if len(proxyConf) == 4 {
			urlProxy.User = url.UserPassword(proxyConf[2], proxyConf[3])
		}
		transport.Proxy = http.ProxyURL(urlProxy)
	} else if viper.GetString("app.proxyType") == "socks5" {
		if len(proxyConf) == 4 {
			proxyUrl = fmt.Sprintf("socks5://%s:%s@%s:%s", proxyConf[2], proxyConf[3], proxyConf[0], proxyConf[1])
		} else {
			proxyUrl = fmt.Sprintf("socks5://%s:%s", proxyConf[0], proxyConf[1])
		}
		urlProxy, _ = urlI.Parse(proxyUrl)
		transport.Proxy = http.ProxyURL(urlProxy)
	}

	return transport
}

type ProxyRoundTripper struct {
	APIKey   string
	ProxyURL string
}

func (t *ProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if t.ProxyURL != "" {
		var proxyUrl string
		var urlProxy *url.URL
		urlI := url.URL{}
		proxyConf := strings.Split(t.ProxyURL, ":")

		if viper.GetString("proxyType") == "socks5" {
			if len(proxyConf) == 4 {
				proxyUrl = fmt.Sprintf("socks5://%s:%s@%s:%s", proxyConf[2], proxyConf[3], proxyConf[0], proxyConf[1])
			} else {
				proxyUrl = fmt.Sprintf("socks5://%s:%s", proxyConf[0], proxyConf[1])
			}
			urlProxy, _ = urlI.Parse(proxyUrl)
		} else {
			proxyUrl = fmt.Sprintf("%s:%s", proxyConf[0], proxyConf[1])
			if !strings.Contains(proxyUrl, "http") {
				proxyUrl = fmt.Sprintf("http://%s", proxyUrl)
			}
			urlProxy, _ = urlI.Parse(proxyUrl)
			if len(proxyConf) == 4 {
				urlProxy.User = url.UserPassword(proxyConf[2], proxyConf[3])
			}
		}

		transport.Proxy = http.ProxyURL(urlProxy)
	}

	newReq := req.Clone(req.Context())
	vals := newReq.URL.Query()
	vals.Set("key", t.APIKey)
	newReq.URL.RawQuery = vals.Encode()

	resp, err := transport.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
