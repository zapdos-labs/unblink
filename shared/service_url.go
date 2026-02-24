package shared

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ServiceURL holds the parsed components of a service URL
type ServiceURL struct {
	Scheme   string // rtsp, http, https
	Host     string // IP or hostname
	Port     int    // Explicit port or default
	Path     string // URL path
	Username string // Optional auth username
	Password string // Optional auth password
}

// DefaultPorts for each protocol
var DefaultPorts = map[string]int{
	"rtsp":  554,
	"http":  80,
	"https": 443,
}

// ParseServiceURL parses a service URL and extracts all components
func ParseServiceURL(serviceURL string) (*ServiceURL, error) {
	// Validate URL is not empty
	if strings.TrimSpace(serviceURL) == "" {
		return nil, fmt.Errorf("service URL cannot be empty")
	}

	// Check for scheme before parsing
	if !strings.Contains(serviceURL, "://") {
		return nil, fmt.Errorf("URL must include a protocol (e.g., rtsp://, http://, https://)")
	}

	// Parse URL
	parsed, err := url.Parse(serviceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate scheme
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "" {
		return nil, fmt.Errorf("URL must include a protocol (e.g., rtsp://, http://)")
	}

	// Validate scheme is supported
	supportedSchemes := map[string]bool{
		"rtsp": true, "http": true, "https": true,
	}
	if !supportedSchemes[scheme] {
		return nil, fmt.Errorf("unsupported protocol '%s', supported protocols are: rtsp, http, https", scheme)
	}

	// Validate host
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL must include a host (IP address or hostname)")
	}

	// Validate host is a valid IP or hostname
	if ip := net.ParseIP(host); ip == nil {
		// Not an IP, check if it's a valid hostname
		if !isValidHostname(host) {
			return nil, fmt.Errorf("invalid host address '%s'", host)
		}
	}

	// Extract port or use default
	port := parsed.Port()
	portNum := 0
	if port != "" {
		portNum, err = strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port '%s'", port)
		}
		if portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
		}
	} else {
		// Use default port for scheme
		var ok bool
		portNum, ok = DefaultPorts[scheme]
		if !ok {
			return nil, fmt.Errorf("no default port for protocol '%s'", scheme)
		}
	}

	// Extract path (ensure it starts with /)
	path := parsed.Path
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Extract username and password
	username := parsed.User.Username()
	password, _ := parsed.User.Password()

	return &ServiceURL{
		Scheme:   scheme,
		Host:     host,
		Port:     portNum,
		Path:     path,
		Username: username,
		Password: password,
	}, nil
}

// isValidHostname checks if a hostname is valid
func isValidHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	// Basic hostname validation - allow localhost, domains, and simple hostnames
	return strings.Contains(host, ".") || host == "localhost"
}
