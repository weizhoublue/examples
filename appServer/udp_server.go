/*
This program implements a simple UDP server.

Main Features:
1. Returns the server's hostname when a UDP packet is received.
2. Returns the client's source IP address.
3. Echoes any data from the client's request.

Usage:
go run udp_server.go -port=<port>

Options:
-h: Display help information
-port: Specify the UDP port for the server to listen on (default is 8080)

Notes:
- The server listens on the specified port.

Testing with netcat (nc) on Linux:
- To test the server, you can use the following netcat commands:
  1. Send a message to the server:
     echo "your data here" | nc -u -w1 localhost 8080
  2. Listen for responses from the server:
     nc -u -l 8080
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// ResponseData represents the structure of the response data
type ResponseData struct {
	ServerHostName   string `json:"ServerHostName"`
	ClientIP         string `json:"ClientIP"`
	ServerIP         string `json:"ServerIP"`
	IPVersion        string `json:"IPVersion"`
	RequestData      string `json:"RequestData"`
	RequestTimestamp string `json:"RequestTimestamp"`
}

func main() {
	// Define command-line flags
	help := flag.Bool("h", false, "Display help information")
	port := flag.String("port", "8080", "Specify the UDP port for the server to listen on")
	flag.Parse()

	// If the -h flag is set, display help information and exit
	if *help {
		flag.Usage()
		return
	}

	// Start the UDP server
	address := fmt.Sprintf(":%s", *port)
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port %s: %v", *port, err)
	}
	defer conn.Close()

	fmt.Printf("UDP server is listening on port %s\n", *port)

	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		go handleUDPRequest(conn, addr, buffer[:n])
	}
}

// handleUDPRequest processes incoming UDP requests
func handleUDPRequest(conn *net.UDPConn, addr *net.UDPAddr, data []byte) {
	serverHostName, err := os.Hostname()
	if err != nil {
		log.Printf("Unable to get hostname: %v", err)
		return
	}

	clientIP := addr.IP.String()
	serverIP, ipVersion := getServerIPAndVersion(addr)

	requestData := string(data)
	log.Printf("Received request from %s with data: %s", clientIP, requestData)

	response := ResponseData{
		ServerHostName:   serverHostName,
		ClientIP:         clientIP,
		ServerIP:         serverIP,
		IPVersion:        ipVersion,
		RequestData:      requestData,
		RequestTimestamp: time.Now().Format(time.RFC3339),
	}

	if err := sendUDPResponse(conn, addr, response); err != nil {
		log.Printf("Unable to send response: %v", err)
	}
}

// getServerIPAndVersion determines the server IP and whether the request is IPv4 or IPv6
func getServerIPAndVersion(addr *net.UDPAddr) (string, string) {
	ip := addr.IP
	if ip.To4() != nil {
		return ip.String(), "IPv4"
	}
	return ip.String(), "IPv6"
}

// sendUDPResponse marshals the response data to JSON and sends it back to the client
func sendUDPResponse(conn *net.UDPConn, addr *net.UDPAddr, response ResponseData) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("unable to marshal response data: %v", err)
	}

	_, err = conn.WriteToUDP(responseJSON, addr)
	if err != nil {
		return fmt.Errorf("unable to send response: %v", err)
	}

	log.Printf("Sent response to %s: %s", addr.String(), responseJSON)
	return nil
}
