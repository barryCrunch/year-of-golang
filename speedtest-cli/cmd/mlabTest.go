package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

type Client struct {
	locateURL     url.URL
	targetServers []measurementServer
}

type TargetURL string

type measurementServer struct {
	machine         string
	downloadTargets []TargetURL
	uploadTargets   []TargetURL
}

type location struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type LocateServer struct {
	Machine  string            `json:"machine"`
	Location location          `json:"location"`
	URLs     map[string]string `json:"urls"`
}

type LocateResp struct {
	Resp []LocateServer `json:"results"`
}

type testResult struct {
	BBRInfo struct {
		BW     int64 `json:"BW"`
		MinRTT int64 `json:"MinRTT"`
	} `json:"BBRInfo"`
}

func (c *Client) fetchTargetServers() error {
	request, err := http.NewRequest("GET", c.locateURL.String(), nil)
	if err != nil {
		log.Fatalf("Failed to construct request: %v", err)
		return err
	}
	client := &http.Client{}
	r, err := client.Do(request)
	if err != nil {
		log.Fatalf("Failed to connect to locate and get target servers: %v", err)
		return err
	}
	respBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("Could not read dat body: %v", err)
		return err
	}

	location := LocateResp{}

	if err := json.Unmarshal(respBody, &location); err != nil {
		log.Fatalf("Could not unmarshal dat data: %v", err)
		return err
	}

	for _, server := range location.Resp {
		tempMachine := measurementServer{
			machine: server.Machine,
		}
		for key, url := range server.URLs {
			if strings.Contains(key, "upload") {
				tempMachine.uploadTargets = append(tempMachine.uploadTargets, TargetURL(url))
			}

			if strings.Contains(key, "download") {
				tempMachine.downloadTargets = append(tempMachine.downloadTargets, TargetURL(url))
			}

		}
		c.targetServers = append(c.targetServers, tempMachine)

	}
	return nil
}

func (t TargetURL) runTest() {
	log.Printf("URL Target: %v\n", t)
	headers := http.Header{}
	headers.Add("Sec-WebSocket-Protocol", "net.measurementlab.ndt.v7")
	headers.Add("User-Agent", "ooniprobe/3.0.0 ndt7-client-go/0.1.0")
	var mostRecentTestResult testResult
	client, _, err := websocket.DefaultDialer.Dial(string(t), headers)
	if err != nil {
		log.Println(err)
	}
	defer client.Close()

	for {
		messageType, message, err := client.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
				log.Printf("Result: %+v", mostRecentTestResult)
				log.Println("Connection Close")
				break
			}
			log.Fatalln(err)
		}

		if messageType == websocket.TextMessage {
			if err := json.Unmarshal(message, &mostRecentTestResult); err != nil {
				log.Fatalln(err)

			}
		}

	}

}

func (c *Client) runTests() error {
	for _, server := range c.targetServers {
		log.Printf("Target Machine: %s\n", server.machine)
		for _, target := range server.downloadTargets {
			target.runTest()
		}

		for _, target := range server.uploadTargets {
			target.runTest()
		}
	}
	return nil
}

func TestSpeed(cmd *cobra.Command, args []string) {
	char := "💩"
	fmt.Println(char)
	locateUrl, err := url.Parse("https://locate.measurementlab.net/v2/nearest/ndt/ndt7")
	if err != nil {
		log.Fatalln(err)
	}
	client := Client{
		locateURL: *locateUrl,
	}

	if err := client.fetchTargetServers(); err != nil {
		log.Fatalln(err)
	}
	if err := client.runTests(); err != nil {
		log.Fatalln(err)
	}
}
