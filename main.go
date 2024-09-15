package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

type Entry struct {
	ID    int    `json:"id"`
	Item  string `json:"item"`
	Completed bool `json:"completed"`
}

// Using var here to allow it to be accessible throughout the package
var dataFile = "data.json"
// Mutex prevents concurrent write access to the file
var mu sync.Mutex

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server: ", err)
		return
	}
	defer l.Close()
	fmt.Println("Server listening on portt 8080")

	// In simpleServer I am using Dial because I was simply setting up a connection here I will be using multiple connections and need to be listening out so use accept
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading request: ", err)
		return
	}

	// parseRequeset(req) takes in req which should be a HTTP request line e.g. "POST /data HTTP/1.1\n" this method will parse it to find the HTTP method and the return method Post and path /data HTTP/1.1\n
	method, path := parseRequest(req)
	// Checks if the HTTP method or path extracted from the request line is empty so invalid requests exits the handleConnection function
	if method == "" || path == "" {
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	// reads and parses HTTP headers from the request
	// Initialises a map to store the header fields and their values
	headers := make(map[string]string)
	for {
		// reads a line from the connection
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading headers:", err)
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		// splits the line into key and value
		parts := strings.SplitN(line, ": ", 2)
		// checks that the line was successfully split int key and value
		if len(parts) == 2 {
			headers[parts[0]] = parts[1]
		}
	}

	// Decides which handler function to call based on the HTTP methos and path
	if method == "GET" && path == "/data" {
		handleGet(conn)
		return
	}

	if method == "POST" && path == "/data" {
		contentLength := 0
		if lengthStr, ok := headers["Content-Length"]; ok {
			fmt.Sscanf(lengthStr, "%d", &contentLength)
		}
		handlePost(conn, reader, contentLength)
		return
	}

	conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
}

// This is required to split the Get or post request from the data
// req is the parameter of type sting which represents HTTP request line (e.g. "GET /data HTTP/1.1")
// this function returns two values type string this will be the HTTP methood and the path
func parseRequest(req string) (string, string) {
	// this splits the req string into a slice of substrings seperated by a whitespace req = "GET /data HTTP/1.1" and parts = ["GET", "/data", "HTTP/1.1"]
	parts := strings.Fields(req)
	// This is a check to makes sure that parts has fewer than 2 elements
	if len(parts) < 2 {
		return "", ""
	}
	// For example - parts[0] wil be GET and parts [/data]
	return parts[0], parts[1]
}

// Handle Get request to retrieve all data from the JSON file
func handleGet(conn net.Conn) {
	mu.Lock()
	defer mu.Unlock()

	// Reads the json file and if it can't it will send a HTTP response to the client
	file, err := os.ReadFile(dataFile)
	if err != nil {
		fmt.Println("Error reading file: ", err)
		// converted to byte slice because it is required by conn.Write
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		return
	}

	conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n"))
	conn.Write(file)
}

// Handle Post request to append data to the JSON file
func handlePost(conn net.Conn, reader *bufio.Reader, contentLength int) {
	// allocates the memory to the correct size
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		fmt.Println("Error reading POST body:", err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	fmt.Println("Received POST body:", string(body))

	var newEntries []Entry
	// json.Unmarshal converts json to go
	// by passing a pointer this allows the function to modify the original. Using pointers is memory efficent so you aren't passing large data structures
	// & is for memory address and * is used for accessing of modigying the value
	err = json.Unmarshal(body, &newEntries)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	var entries []Entry
	mu.Lock()
	defer mu.Unlock()

	file, err := os.ReadFile(dataFile)
	if err == nil {
		_ = json.Unmarshal(file, &entries)
	}

	// Assign IDs to new entries and append to existing entries
	for i := range newEntries {
		newEntries[i].ID = len(entries) + i + 1
	}
	entries = append(entries, newEntries...)

	// MarshallIndent does the same as marshall but just gets everything in the right format
	file, err = json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		return
	}
	err = os.WriteFile(dataFile, file, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		return
	}

	conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))
}
