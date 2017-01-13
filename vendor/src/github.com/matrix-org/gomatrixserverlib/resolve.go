package gomatrixserverlib

import (
	"net"
	"strconv"
	"strings"
)

// A HostResult is the result of looking up the IP addresses for a host.
type HostResult struct {
	CName string   // The canonical name for the host.
	Addrs []string // The IP addresses for the host.
	Error error    // If there was an error getting the IP addresses.
}

// A DNSResult is the result of looking up a matrix server in DNS.
type DNSResult struct {
	SRVCName   string                // The canonical name for the SRV record in DNS
	SRVRecords []*net.SRV            // List of SRV record for the matrix server.
	SRVError   error                 // If there was an error getting the SRV records.
	Hosts      map[string]HostResult // The results of looking up the SRV record targets.
	Addrs      []string              // List of "<ip>:<port>" strings that the server is listening on. These strings can be passed to `net.Dial()`.
}

// LookupServer looks up a matrix server in DNS.
func LookupServer(serverName string) (*DNSResult, error) {
	var result DNSResult
	result.Hosts = map[string]HostResult{}

	hosts := map[string][]net.SRV{}
	if strings.Index(serverName, ":") == -1 {
		// If there isn't an explicit port set then try to look up the SRV record.
		var err error
		result.SRVCName, result.SRVRecords, err = net.LookupSRV("matrix", "tcp", serverName)
		result.SRVError = err

		if err != nil {
			if dnserr, ok := err.(*net.DNSError); ok {
				// If the error is a network timeout talking to the DNS server
				// then give up now rather than trying to fallback.
				if dnserr.Timeout() {
					return nil, err
				}
				// If there isn't a SRV record in DNS then fallback to "serverName:8448".
				hosts[serverName] = []net.SRV{net.SRV{
					Target: serverName,
					Port:   8448,
				}}
			}
		} else {
			// Group the SRV records by target host.
			for _, record := range result.SRVRecords {
				hosts[record.Target] = append(hosts[record.Target], *record)
			}
		}
	} else {
		// There is a explicit port set in the server name.
		// We don't need to look up any SRV records.
		host, portStr, err := net.SplitHostPort(serverName)
		if err != nil {
			return nil, err
		}
		var port uint64
		port, err = strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return nil, err
		}
		hosts[host] = []net.SRV{net.SRV{
			Target: host,
			Port:   uint16(port),
		}}
	}
	// Look up the IP addresses for each host.
	for host, records := range hosts {
		// Ignore any DNS errors when looking up the CNAME. We only are interested in it for debugging.
		cname, _ := net.LookupCNAME(host)
		addrs, err := net.LookupHost(host)
		result.Hosts[host] = HostResult{
			CName: cname,
			Addrs: addrs,
			Error: err,
		}
		// For each SRV record, for each IP address add a "<ip>:<port>" entry to the list of addresses.
		for _, record := range records {
			for _, addr := range addrs {
				ipPort := net.JoinHostPort(addr, strconv.Itoa(int(record.Port)))
				result.Addrs = append(result.Addrs, ipPort)
			}
		}
	}

	return &result, nil
}
