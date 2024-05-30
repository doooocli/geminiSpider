package main

import (
	"context"
	"fmt"
	"golang.org/x/net/proxy"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var testApi = "https://api.ipify.org/?format=json"

func test() {
	//us.922s5.net:6300:24713127-zone-custom-region-US-sessid-3zCrHfeh:uEaufCYO
	var proxyIP = "us.922s5.net:6300"
	var username = "24713127-zone-custom-region-US-sessid-3zCrHfeh"
	var password = "uEaufCYO"

	httpProxy(proxyIP, username, password)
	time.Sleep(time.Second * 1)
	//Socks5Proxy(proxyIP, username, password)

}
func httpProxy(proxyUrl, user, pass string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	urli := url.URL{}

	if !strings.Contains(proxyUrl, "http") {
		proxyUrl = fmt.Sprintf("http://%s", proxyUrl)
	}

	urlProxy, _ := urli.Parse(proxyUrl)
	if user != "" && pass != "" {
		urlProxy.User = url.UserPassword(user, pass)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(urlProxy),
		},
	}
	rqt, err := http.NewRequest("GET", testApi, nil)
	if err != nil {
		panic(err)
		return
	}
	response, err := client.Do(rqt)
	if err != nil {
		panic(err)
		return
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)
	fmt.Println("http result = ", response.Status, string(body))

	return
}

func Socks5Proxy(proxyUrl, user, pass string) {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	var userAuth proxy.Auth
	if user != "" && pass != "" {
		userAuth.User = user
		userAuth.Password = pass
	}
	dialer, err := proxy.SOCKS5("tcp", proxyUrl, &userAuth, proxy.Direct)
	if err != nil {
		panic(err)
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				return dialer.Dial(network, addr)
			},
		},
		Timeout: time.Second * 10,
	}

	if resp, err := httpClient.Get(testApi); err != nil {
		panic(err)
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("socks5 result = ", string(body))
	}
}
