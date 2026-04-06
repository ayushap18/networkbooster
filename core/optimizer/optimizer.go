package optimizer

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Tweak represents a single network optimization.
type Tweak struct {
	Name        string
	Description string
	Applied     bool
	Before      string
	After       string
	Error       string
}

// Result contains the results of running the optimizer.
type Result struct {
	Tweaks    []Tweak
	DNSBefore string
	DNSAfter  string
	Platform  string
}

// dnsCandidate holds a DNS server and its measured latency.
type dnsCandidate struct {
	Name    string
	Primary string
	Secondary string
	Latency time.Duration
}

var dnsCandidates = []dnsCandidate{
	{Name: "Cloudflare", Primary: "1.1.1.1", Secondary: "1.0.0.1"},
	{Name: "Google", Primary: "8.8.8.8", Secondary: "8.8.4.4"},
	{Name: "Quad9", Primary: "9.9.9.9", Secondary: "149.112.112.112"},
	{Name: "OpenDNS", Primary: "208.67.222.222", Secondary: "208.67.220.220"},
	{Name: "AdGuard", Primary: "94.140.14.14", Secondary: "94.140.15.15"},
}

// tcpTweak defines a sysctl parameter to tune.
type tcpTweak struct {
	Key         string
	Value       string
	Name        string
	Description string
}

// macOS TCP optimizations for maximum throughput.
var macTCPTweaks = []tcpTweak{
	{
		Key: "net.inet.tcp.delayed_ack", Value: "0",
		Name: "Disable TCP Delayed ACK",
		Description: "Send ACKs immediately — reduces latency for interactive traffic",
	},
	{
		Key: "net.inet.tcp.sendspace", Value: "262144",
		Name: "Increase TCP Send Buffer",
		Description: "256KB send buffer (was 128KB) — more data in flight for uploads",
	},
	{
		Key: "net.inet.tcp.recvspace", Value: "262144",
		Name: "Increase TCP Receive Buffer",
		Description: "256KB receive buffer (was 128KB) — more data in flight for downloads",
	},
	{
		Key: "net.inet.tcp.autorcvbufmax", Value: "8388608",
		Name: "Increase Auto Receive Buffer Max",
		Description: "8MB max auto-tuned receive buffer (was 4MB) — better for high-bandwidth connections",
	},
	{
		Key: "net.inet.tcp.autosndbufmax", Value: "8388608",
		Name: "Increase Auto Send Buffer Max",
		Description: "8MB max auto-tuned send buffer (was 4MB) — better for uploads",
	},
	{
		Key: "net.inet.tcp.mssdflt", Value: "1460",
		Name: "Optimize TCP MSS",
		Description: "Full-size segments (was 512) — fewer packets per MB transferred",
	},
	{
		Key: "net.inet.tcp.win_scale_factor", Value: "8",
		Name: "Increase TCP Window Scale",
		Description: "Larger TCP window (was 3) — allows more data in flight on fast links",
	},
}

// Optimize runs all network optimizations. Requires root for sysctl and DNS changes.
func Optimize() Result {
	result := Result{Platform: runtime.GOOS}

	if runtime.GOOS != "darwin" {
		result.Tweaks = append(result.Tweaks, Tweak{
			Name:  "Platform Check",
			Error: "optimizer currently supports macOS only",
		})
		return result
	}

	// 1. Find and set fastest DNS
	dnsTweak := optimizeDNS()
	result.Tweaks = append(result.Tweaks, dnsTweak)
	result.DNSBefore = dnsTweak.Before
	result.DNSAfter = dnsTweak.After

	// 2. Apply TCP optimizations
	for _, t := range macTCPTweaks {
		tweak := applySysctl(t)
		result.Tweaks = append(result.Tweaks, tweak)
	}

	// 3. Flush DNS cache
	flushTweak := flushDNS()
	result.Tweaks = append(result.Tweaks, flushTweak)

	return result
}

// Reset restores default macOS network settings.
func Reset() Result {
	result := Result{Platform: runtime.GOOS}

	defaults := []tcpTweak{
		{Key: "net.inet.tcp.delayed_ack", Value: "3", Name: "Restore TCP Delayed ACK"},
		{Key: "net.inet.tcp.sendspace", Value: "131072", Name: "Restore TCP Send Buffer"},
		{Key: "net.inet.tcp.recvspace", Value: "131072", Name: "Restore TCP Receive Buffer"},
		{Key: "net.inet.tcp.autorcvbufmax", Value: "4194304", Name: "Restore Auto Receive Buffer Max"},
		{Key: "net.inet.tcp.autosndbufmax", Value: "4194304", Name: "Restore Auto Send Buffer Max"},
		{Key: "net.inet.tcp.mssdflt", Value: "512", Name: "Restore TCP MSS"},
		{Key: "net.inet.tcp.win_scale_factor", Value: "3", Name: "Restore TCP Window Scale"},
	}

	for _, t := range defaults {
		tweak := applySysctl(t)
		result.Tweaks = append(result.Tweaks, tweak)
	}

	// Reset DNS to automatic (empty = use DHCP)
	iface := getActiveInterface()
	if iface != "" {
		out, err := exec.Command("sudo", "networksetup", "-setdnsservers", iface, "empty").CombinedOutput()
		tweak := Tweak{Name: "Reset DNS to automatic", Description: "Use DHCP-provided DNS"}
		if err != nil {
			tweak.Error = fmt.Sprintf("%s: %v", strings.TrimSpace(string(out)), err)
		} else {
			tweak.Applied = true
			tweak.After = "automatic (DHCP)"
		}
		result.Tweaks = append(result.Tweaks, tweak)
	}

	return result
}

func optimizeDNS() Tweak {
	tweak := Tweak{
		Name:        "Optimize DNS",
		Description: "Test DNS resolvers and set the fastest one",
	}

	iface := getActiveInterface()
	if iface == "" {
		tweak.Error = "could not detect active network interface"
		return tweak
	}

	// Get current DNS
	out, _ := exec.Command("networksetup", "-getdnsservers", iface).CombinedOutput()
	tweak.Before = strings.TrimSpace(string(out))

	// Test all DNS candidates concurrently
	type testResult struct {
		idx     int
		latency time.Duration
		ok      bool
	}
	results := make(chan testResult, len(dnsCandidates))

	for i, c := range dnsCandidates {
		go func(idx int, dns dnsCandidate) {
			lat := testDNS(dns.Primary)
			results <- testResult{idx: idx, latency: lat, ok: lat > 0}
		}(i, c)
	}

	var tested []struct {
		idx     int
		latency time.Duration
	}
	for range dnsCandidates {
		r := <-results
		if r.ok {
			tested = append(tested, struct {
				idx     int
				latency time.Duration
			}{r.idx, r.latency})
		}
	}

	if len(tested) == 0 {
		tweak.Error = "no DNS servers reachable"
		return tweak
	}

	sort.Slice(tested, func(i, j int) bool {
		return tested[i].latency < tested[j].latency
	})

	fastest := dnsCandidates[tested[0].idx]
	tweak.After = fmt.Sprintf("%s (%s, %s) — %dms",
		fastest.Name, fastest.Primary, fastest.Secondary,
		fastest.Latency.Milliseconds())

	// Apply DNS change
	out, err := exec.Command("sudo", "networksetup", "-setdnsservers", iface,
		fastest.Primary, fastest.Secondary).CombinedOutput()
	if err != nil {
		tweak.Error = fmt.Sprintf("failed to set DNS: %s: %v", strings.TrimSpace(string(out)), err)
		return tweak
	}

	tweak.Applied = true
	return tweak
}

func testDNS(server string) time.Duration {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "udp", server+":53")
		},
	}

	// Resolve a known domain and measure time
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	_, err := resolver.LookupHost(ctx, "www.google.com")
	if err != nil {
		return 0
	}
	return time.Since(start)
}

func applySysctl(t tcpTweak) Tweak {
	tweak := Tweak{
		Name:        t.Name,
		Description: t.Description,
	}

	// Read current value
	out, _ := exec.Command("sysctl", "-n", t.Key).CombinedOutput()
	tweak.Before = strings.TrimSpace(string(out))

	if tweak.Before == t.Value {
		tweak.Applied = true
		tweak.After = t.Value + " (already set)"
		return tweak
	}

	// Apply new value
	out, err := exec.Command("sudo", "sysctl", "-w", t.Key+"="+t.Value).CombinedOutput()
	if err != nil {
		tweak.Error = fmt.Sprintf("%s: %v", strings.TrimSpace(string(out)), err)
		return tweak
	}

	tweak.Applied = true
	tweak.After = t.Value
	return tweak
}

func flushDNS() Tweak {
	tweak := Tweak{
		Name:        "Flush DNS Cache",
		Description: "Clear stale DNS entries for fresh lookups",
	}

	out, err := exec.Command("sudo", "dscacheutil", "-flushcache").CombinedOutput()
	if err != nil {
		tweak.Error = fmt.Sprintf("%s: %v", strings.TrimSpace(string(out)), err)
		return tweak
	}

	// Also restart mDNSResponder
	exec.Command("sudo", "killall", "-HUP", "mDNSResponder").Run()

	tweak.Applied = true
	tweak.After = "cache flushed"
	return tweak
}

func getActiveInterface() string {
	// Try common interface names
	out, err := exec.Command("networksetup", "-listallnetworkservices").CombinedOutput()
	if err != nil {
		return "Wi-Fi"
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		// Check if this interface is active
		status, _ := exec.Command("networksetup", "-getinfo", line).CombinedOutput()
		if strings.Contains(string(status), "IP address:") &&
			!strings.Contains(string(status), "IP address: none") {
			return line
		}
	}

	return "Wi-Fi"
}
