package ops

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/proxy"
)

// ProxyConfig holds proxy rotation configuration.
type ProxyConfig struct {
	Enabled  bool
	Proxies  []string // list of proxy URLs (socks5://host:port or http://host:port)
	Mode     ProxyMode
	counter  uint64
}

// ProxyMode defines how proxies are selected.
type ProxyMode int

const (
	ProxyRoundRobin ProxyMode = iota
	ProxyRandom
)

// NoProxy returns a disabled proxy config.
func NoProxy() ProxyConfig {
	return ProxyConfig{Enabled: false}
}

// LoadProxies loads proxy list from a file (one per line).
func LoadProxies(filename string) (*ProxyConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open proxy file: %w", err)
	}
	defer f.Close()

	var proxies []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			proxies = append(proxies, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read proxy file: %w", err)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies found in %s", filename)
	}

	return &ProxyConfig{
		Enabled: true,
		Proxies: proxies,
		Mode:    ProxyRoundRobin,
	}, nil
}

// Next returns the next proxy URL based on the configured mode.
func (pc *ProxyConfig) Next() string {
	if !pc.Enabled || len(pc.Proxies) == 0 {
		return ""
	}
	switch pc.Mode {
	case ProxyRandom:
		return pc.Proxies[rand.Intn(len(pc.Proxies))]
	default: // RoundRobin
		idx := atomic.AddUint64(&pc.counter, 1)
		return pc.Proxies[idx%uint64(len(pc.Proxies))]
	}
}

// Count returns the number of loaded proxies.
func (pc *ProxyConfig) Count() int {
	return len(pc.Proxies)
}

// NewHTTPTransport creates an http.Transport using the given proxy URL.
func NewHTTPTransport(proxyURL string) (*http.Transport, error) {
	if proxyURL == "" {
		return http.DefaultTransport.(*http.Transport).Clone(), nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}

	switch parsed.Scheme {
	case "socks5", "socks5h":
		auth := &proxy.Auth{}
		if parsed.User != nil {
			auth.User = parsed.User.Username()
			auth.Password, _ = parsed.User.Password()
		} else {
			auth = nil
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 dialer: %w", err)
		}
		return &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}, nil
	case "http", "https":
		return &http.Transport{
			Proxy: http.ProxyURL(parsed),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}
}

// ProxyPool manages a thread-safe pool of proxy transports.
type ProxyPool struct {
	config     *ProxyConfig
	transports sync.Map // proxyURL → *http.Transport
}

// NewProxyPool creates a proxy pool from config.
func NewProxyPool(config *ProxyConfig) *ProxyPool {
	return &ProxyPool{config: config}
}

// GetTransport returns an http.Transport for the next proxy.
func (pp *ProxyPool) GetTransport() (*http.Transport, string, error) {
	if pp.config == nil || !pp.config.Enabled {
		return http.DefaultTransport.(*http.Transport).Clone(), "", nil
	}

	proxyURL := pp.config.Next()
	if cached, ok := pp.transports.Load(proxyURL); ok {
		return cached.(*http.Transport), proxyURL, nil
	}

	transport, err := NewHTTPTransport(proxyURL)
	if err != nil {
		return nil, proxyURL, err
	}

	pp.transports.Store(proxyURL, transport)
	return transport, proxyURL, nil
}

// ModeName returns a display string for the proxy mode.
func (pc *ProxyConfig) ModeName() string {
	if !pc.Enabled {
		return "OFF"
	}
	switch pc.Mode {
	case ProxyRandom:
		return "Random"
	default:
		return "Round-Robin"
	}
}
