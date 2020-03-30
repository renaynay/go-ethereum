package node

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
)

// TODO do I need tests for the other handlers? (ex. cors, vhosts, gzip)?

func TestNewHTTPHandlerStack_Cors(t *testing.T) {
	// todo
	srv := rpc.NewServer()
	cors := []string{}
	vhosts := []string{}

	handler := NewHTTPHandlerStack(srv, cors, vhosts)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	responses := make(chan *http.Response)
	go func(responses chan *http.Response) {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", ts.URL, nil)
		//req.Header.Set("Content-type", "application/json")
		// req.Header.Set() todo



		resp, err := client.Do(req)
		if err != nil {
			t.Error("could not issue a GET request to the test http server") // TODO improve error message?
		}
		responses <- resp
	}(responses)

	response := <- responses
	assert.Equal(t,"websocket", response.Header.Get("Upgrade"))
}

func TestNewWebsocketUpgradeHandler_websocket(t *testing.T) {
	srv := rpc.NewServer()

	handler := NewWebsocketUpgradeHandler(nil, srv.WebsocketHandler([]string{}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	responses := make(chan *http.Response)
	go func(responses chan *http.Response) {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", ts.URL, nil)
		req.Header.Set("Connection", "upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-Websocket-Key", "SGVsbG8sIHdvcmxkIQ==")

		resp, err := client.Do(req)
		if err != nil {
			t.Error("could not issue a GET request to the test http server") // TODO improve error message?
		}
		responses <- resp
	}(responses)

	response := <- responses
	assert.Equal(t,"websocket", response.Header.Get("Upgrade"))
}
