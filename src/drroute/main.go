package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const InternalServerError = "500"

var PIDFILE_LOC = "/var/vcap/sys/run/drroute/drroute.pid"
var stopChan chan struct{}

func main() {
	fmt.Println("Starting dr. Route....")
	http.HandleFunc("/start", start)
	http.HandleFunc("/stop", stop)
	http.HandleFunc("/health", health)
	fmt.Println("Doctor route running...")
	err := writePidFile(PIDFILE_LOC)
	if err != nil {
		log.Fatal("Error writing pidfile: ", err)
	}
	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatal("Error listening: ", err)
	}
	os.Exit(0)
}

func writePidFile(pidFile string) error {
	if pidFile != "" {
		pid := strconv.Itoa(os.Getpid())
		err := ioutil.WriteFile(pidFile, []byte(pid), 0660)
		if err != nil {
			return fmt.Errorf("cannot create pid file:  %v", err)
		}
	}
	return nil
}

type Results struct {
	TotalRequests int
	Responses     map[string]int
}

type StartRequest struct {
	Endpoint string
}

type Poller interface {
	Poll(uri string) string
}

type httpPoller struct {
}

type tcpPoller struct {
}

func (h *httpPoller) Poll(url string) string {
	var statusCode string
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error connecting to app: %s\n", err.Error())
		statusCode = InternalServerError
	}
	defer resp.Body.Close()

	statusCode = strconv.Itoa(resp.StatusCode)
	return statusCode
}

func (h *tcpPoller) Poll(endpoint string) string {
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	if err != nil {
		fmt.Printf("Error connecting to app: %s\n", err.Error())
		return InternalServerError
	}

	defer conn.Close()
	message := []byte(fmt.Sprintf("GET /health HTTP/1.1\nHost: %s\n\n", endpoint))
	_, err = conn.Write(message)
	if err != nil {
		fmt.Printf("Error writing HTTP req: %s\n", err.Error())
		return InternalServerError
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n <= 0 {
		fmt.Printf("Error reading HTTP response: %s\n", err.Error())
		return InternalServerError
	}

	body := string(buf[:n])

	fmt.Printf("body: %s\n", body)

	parts := strings.Split(body, " ")
	if len(parts) > 1 {
		statusCode := parts[1]
		fmt.Printf("statusCode: %s\n", statusCode)
		return statusCode
	}

	return InternalServerError
}

var runResults Results

func health(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		fmt.Println("Checking health...")
		payload, err := json.Marshal(runResults)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
		res.Write(payload)
	} else {
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func stop(res http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		fmt.Println("Stopping...")
		if stopChan != nil {
			stopChan <- struct{}{}
		}
		res.WriteHeader(http.StatusNoContent)
	} else {
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func start(res http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		if stopChan != nil {
			http.Error(res, "Already started!", http.StatusBadRequest)
			return
		}
		fmt.Println("Creating stop channel")
		stopChan = make(chan struct{})

		fmt.Println("Starting...")
		var startRequest StartRequest
		payload, err := ioutil.ReadAll(req.Body)
		if err != nil {
			fmt.Println("Error while readin request", err.Error())
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(payload, &startRequest)
		if err != nil {
			fmt.Println("Error while decoding request", err.Error())
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		if startRequest.Endpoint == "" {
			startRequest.Endpoint = req.Host
		}

		// i.e. http://foo.com/health or foo.com:9000
		url := startRequest.Endpoint

		var poller Poller
		if strings.HasPrefix(url, "http://") {
			poller = &httpPoller{}
		} else {
			poller = &tcpPoller{}
		}

		fmt.Println("Endpoint to poll", url)
		go func() {
			// polling = true
			runResults = Results{}
			runResults.Responses = make(map[string]int)
			for i := 1; ; i++ {
				select {
				default:
					fmt.Printf("Poll [%d]...\n", i)
					statusCode := poller.Poll(url)
					count, ok := runResults.Responses[statusCode]
					if !ok {
						count = 0
					}
					runResults.Responses[statusCode] = count + 1
					runResults.TotalRequests = i
					time.Sleep(1 * time.Second)
				case <-stopChan:
					close(stopChan)
					stopChan = nil
					fmt.Println("Request to stop polling..")
					return
				}
			}
		}()
		res.WriteHeader(http.StatusNoContent)
	} else {
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}
