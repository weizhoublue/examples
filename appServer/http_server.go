/*
This program implements a simple HTTP server.

Main Features:
1. Returns the server's hostname when an HTTP request is received.
2. Returns the client's source IP address.
3. Echoes any data from the client's request.

Usage:
go run http_server.go -port=<port>

Options:
-h: Display help information
-port: Specify the TCP port for the server to listen on (default is 8080)

Notes:
- The server listens on the specified port.

Testing with curl:
- To test the server over IPv4, use:
  curl http://127.0.0.1:8080
- To test the server over IPv6, use:
  curl http://[::1]:8080
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

// ResponseData represents the structure of the response data
type ResponseData struct {
	ServerHostName     string            `json:"ServerHostName"`
	ClientIP           string            `json:"ClientIP"`
	ServerIP           string            `json:"ServerIP"`
	IPVersion          string            `json:"IPVersion"`
	RequestData        string            `json:"RequestData"`
	RequestHttpHeaders map[string]string `json:"RequestHttpHeaders"`
	RequestTimestamp   string            `json:"RequestTimestamp"`
}

func main() {
	// Define command-line flags
	help := flag.Bool("h", false, "Display help information")
	port := flag.String("port", "8080", "Specify the TCP port for the server to listen on")
	flag.Parse()

	// If the -h flag is set, display help information and exit
	if *help {
		flag.Usage()
		return
	}

	http.HandleFunc("/", handleRequest)

	// Start the HTTP server
	address := fmt.Sprintf(":%s", *port)
	fmt.Printf("Server is listening on port %s\n", *port)
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}

// handleRequest processes incoming HTTP requests
func handleRequest(w http.ResponseWriter, r *http.Request) {
	serverHostName, clientIP, serverIP, ipVersion, requestData, requestHttpHeaders, err := processRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ResponseData{
		ServerHostName:     serverHostName,
		ClientIP:           clientIP,
		ServerIP:           serverIP,
		IPVersion:          ipVersion,
		RequestData:        requestData,
		RequestHttpHeaders: requestHttpHeaders,
		RequestTimestamp:   time.Now().Format(time.RFC3339),
	}

	if err := sendResponse(w, response); err != nil {
		http.Error(w, "Unable to send response", http.StatusInternalServerError)
	}
}

// processRequest extracts and logs request data
func processRequest(r *http.Request) (string, string, string, string, string, map[string]string, error) {
	serverHostName, err := os.Hostname()
	if err != nil {
		return "", "", "", "", "", nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", "", "", "", "", nil, fmt.Errorf("unable to parse client IP address: %v", err)
	}

	serverIP, ipVersion := getLocalIPAndVersion()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", "", "", "", "", nil, fmt.Errorf("unable to read request body: %v", err)
	}

	requestHttpHeaders := make(map[string]string)
	for name, values := range r.Header {
		requestHttpHeaders[name] = values[0] // Assuming single value for simplicity
	}

	requestData := string(body)
	log.Printf("Received request from %s with data: %s", clientIP, requestData)

	return serverHostName, clientIP, serverIP, ipVersion, requestData, requestHttpHeaders, nil
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

// sendResponse marshals the response data to JSON and writes it to the response writer
func sendResponse(w http.ResponseWriter, response ResponseData) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("unable to marshal response data: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)

	log.Printf("Sent response: %s", responseJSON)
	return nil
}
