package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

var (
	client    *http.Client
	proxyList = `https://raw.githubusercontent.com/fate0/proxylist/master/proxy.list`
	sema      = semaphore.NewWeighted(5)
	ctx       = context.TODO()
	wg        sync.WaitGroup
)

type ProxyItem struct {
	Port    json.Number `json:"port"`
	Type    string      `json:"type"`
	Host    string      `json:"host"`
	Country string      `json:"country"`
}

func validate(pi ProxyItem) bool {
	defer func() {
		sema.Release(1)
		wg.Done()
	}()
	proxyString := fmt.Sprintf("%s://%s:%s", pi.Type, pi.Host, pi.Port)
	proxyURL, _ := url.Parse(proxyString)

	c := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	req, err := http.NewRequest("GET", "http://ip.cn", nil)
	if err != nil {
		fmt.Println(err)
		return false
	}

	req.Header.Set("User-Agent", "curl/7.54.0")
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if len(content) > 200 {
		fmt.Println("too long response is treated as unexpected response")
		return false
	}
	c = nil
	fmt.Printf("%s://%s:%s, %s, %s", pi.Type, pi.Host, string(pi.Port), pi.Country, string(content))
	return true
}

func main() {
	client = &http.Client{
		Timeout: 120 * time.Second,
	}

	retry := 0
	req, err := http.NewRequest("GET", proxyList, nil)
	if err != nil {
		log.Println("proxy list - Could not parse proxy list request:", err)
		return
	}
doRequest:
	resp, err := client.Do(req)
	if err != nil {
		log.Println("proxy list - Could not send proxy list request:", err)
		retry++
		if retry < 3 {
			time.Sleep(3 * time.Second)
			goto doRequest
		}
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("proxy list - proxy list request not 200")
		retry++
		if retry < 3 {
			time.Sleep(3 * time.Second)
			goto doRequest
		}
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	var pi ProxyItem
	for scanner.Scan() {
		line := scanner.Text()
		if err := json.Unmarshal([]byte(line), &pi); err == nil {
			sema.Acquire(ctx, 1)
			wg.Add(1)
			go validate(pi)
		} else {
			fmt.Printf("%s, %v\n", line, err)
		}
	}
	wg.Wait()
}
