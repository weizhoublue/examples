/*
This program implements a simple HTTP proxy server.

Main Features:
1. Forwards client requests to a specified backend URL.
2. Controls the timeout for backend requests.
3. Returns the backend response to the client, including success status and data or error message.

Usage:
go run proxy_http_server.go -port=<port> -timeout=<seconds>

Options:
-h: Display help information
-port: Specify the TCP port for the server to listen on (default is 8090)
-timeout: Specify the default timeout for backend requests in seconds (default is 4)

Notes:
- The server listens on the specified port.

Testing with curl:
- To test the proxy server over IPv4, use:
  curl -X POST http://127.0.0.1:8090 -d '{"BackendUrl":"http://example.com","Timeout":5}' -H "Content-Type: application/json"
- To test the proxy server over IPv6, use:
  curl -X POST http://[::1]:8090 -d '{"BackendUrl":"http://example.com","Timeout":5}' -H "Content-Type: application/json"
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// ClientRequest represents the structure of the client's request body
type ClientRequest struct {
	BackendUrl string `json:"BackendUrl"`
	Timeout    int    `json:"Timeout"`
}

// ProxyResponse represents the structure of the response data
type ProxyResponse struct {
	Success       bool   `json:"Success"`
	BackendData   string `json:"BackendData,omitempty"`
	ErrorMessage  string `json:"ErrorMessage,omitempty"`
	ProxyHostName string `json:"ProxyHostName"`
	ClientIP      string `json:"ClientIP"`
	ProxyServerIP string `json:"ProxyServerIP"`
	IPVersion     string `json:"IPVersion"`
}

func main() {
	// Define command-line flags
	help := flag.Bool("h", false, "Display help information")
	port := flag.String("port", "8090", "Specify the TCP port for the server to listen on")
	defaultTimeout := flag.Int("timeout", 4, "Specify the default timeout for backend requests in seconds")
	flag.Parse()

	// If the -h flag is set, display help information and exit
	if *help {
		flag.Usage()
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var clientReq ClientRequest
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &clientReq); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		if clientReq.BackendUrl == "" {
			http.Error(w, "BackendUrl is required", http.StatusBadRequest)
			return
		}

		timeout := time.Duration(clientReq.Timeout) * time.Second
		if clientReq.Timeout == 0 {
			timeout = time.Duration(*defaultTimeout) * time.Second
		}

		client := &http.Client{Timeout: timeout}

		resp, err := client.Get(clientReq.BackendUrl)
		if err != nil {
			sendProxyResponse(w, r, ProxyResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to access backend: %v", err),
			})
			return
		}
		defer resp.Body.Close()

		backendData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sendProxyResponse(w, r, ProxyResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read backend response: %v", err),
			})
			return
		}

		sendProxyResponse(w, r, ProxyResponse{
			Success:     true,
			BackendData: string(backendData),
		})
	})

	// Start the HTTP server
	address := fmt.Sprintf(":%s", *port)
	fmt.Printf("Proxy server is listening on port %s\n", *port)
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}

// sendProxyResponse marshals the response data to JSON and writes it to the response writer
func sendProxyResponse(w http.ResponseWriter, r *http.Request, response ProxyResponse) {
	hostname, _ := os.Hostname()
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	serverIP, ipVersion := getLocalIPAndVersion()

	response.ProxyHostName = hostname
	response.ClientIP = clientIP
	response.ProxyServerIP = serverIP
	response.IPVersion = ipVersion

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Unable to marshal response data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)

	log.Printf("Sent response: %s", responseJSON)
}

// getLocalIPAndVersion determines the server's local IP and whether it is IPv4 or IPv6
func getLocalIPAndVersion() (string, string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", "Unknown"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), "IPv4"
			}
			if ipNet.IP.To16() != nil {
				return ipNet.IP.String(), "IPv6"
			}
		}
	}
	return "", "Unknown"
}
