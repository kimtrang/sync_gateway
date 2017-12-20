package rest

import (
	"log"
	"net/http/httptest"
	"testing"

	"bytes"
	"fmt"
	"net/http"
	"net/url"

	"github.com/couchbase/go-blip"
	"github.com/couchbase/sync_gateway/base"
	"github.com/couchbaselabs/go.assert"
	"time"
	"encoding/json"
)

func TestBlipEndpointHttpTest(t *testing.T) {

	var rt RestTester
	defer rt.Close()

	base.EnableLogKey("HTTP")
	base.EnableLogKey("HTTP+")
	base.EnableLogKey("BLIP")
	base.EnableLogKey("BLIP+")
	base.EnableLogKey("Sync")
	base.EnableLogKey("Sync+")

	// Create an admin handler
	adminHandler := rt.TestAdminHandler()

	// Create a test server and close it when the test is complete
	srv := httptest.NewServer(adminHandler)
	defer srv.Close()

	// Construct URL to connect to blipsync target endpoint
	destUrl := fmt.Sprintf("%s/db/_blipsync", srv.URL)
	u, err := url.Parse(destUrl)
	assertNoError(t, err, "Error parsing desturl")
	u.Scheme = "ws"

	// Make BLIP/Websocket connection
	blipContext := blip.NewContext()
	blipContext.Logger = func(fmt string, params ...interface{}) {
		base.LogTo("BLIP", fmt, params...)
	}
	blipContext.LogMessages = true
	blipContext.LogFrames = true
	origin := "http://localhost" // TODO: what should be used here?
	sender, err := blipContext.Dial(u.String(), origin)
	assertNoError(t, err, "Websocket connection error")


	// When this test sends subChanges, Sync Gateway will send a changes request that must be handled
	blipContext.HandlerForProfile["changes"] = func(request *blip.Message) {
		log.Printf("got changes message: %+v", request)
		body, err := request.Body()
		log.Printf("changes body: %v, err: %v", string(body), err)
	}

	// When this test sends changes, Sync Gateway will send rev
	blipContext.HandlerForProfile["rev"] = func(request *blip.Message) {
		log.Printf("got rev message: %+v", request)
		body, err := request.Body()
		log.Printf("rev body: %v, err: %v", string(body), err)
	}


	// Verify the server will accept the change
	var changeList [][]interface{}
	changesRequest := blip.NewRequest()
	changesRequest.SetProfile("changes")
	changesRequest.SetBody([]byte(`[["1", "foo", "1-abc", false]]`)) // [sequence, docID, revID]
	sent := sender.Send(changesRequest)
	assert.True(t, sent)
	changesResponse := changesRequest.Response()
	assert.Equals(t, changesResponse.SerialNumber(), changesRequest.SerialNumber())
	body, err := changesResponse.Body()
	assertNoError(t, err, "Error reading changes response body")
	err = json.Unmarshal(body, &changeList)
	assertNoError(t, err, "Error unmarshalling response body")
	log.Printf("changes response body: %s", body)
	assert.True(t, len(changeList) == 1)  // Should be 1 row, corresponding to the single doc that was queried in changes
	changeRow := changeList[0]
	assert.True(t, len(changeRow) == 0)  // Should be empty, meaning the server is saying it doesn't have the revision yet

	// Send the change in a rev request
	revRequest := blip.NewRequest()
	revRequest.SetProfile("rev")
	revRequest.Properties["id"] = "foo"
	revRequest.Properties["rev"] = "1-abc"
	revRequest.Properties["deleted"] = "false"
	revRequest.Properties["sequence"] = "1"
	revRequest.SetBody([]byte(`{"key": "val"}`))
	sent = sender.Send(revRequest)
	assert.True(t, sent)
	revResponse := revRequest.Response()
	assert.Equals(t, revResponse.SerialNumber(), revRequest.SerialNumber())
	body, err = revResponse.Body()
	assertNoError(t, err, "Error unmarshalling response body")
	log.Printf("rev response body: %s", body)


	//// get changes, should be empty
	//subChangesRequest := blip.NewRequest()
	//subChangesRequest.SetProfile("subChanges")
	//sent = sender.Send(subChangesRequest)
	//assert.True(t, sent)
	//
	//log.Printf("sender: %v", sender)
	//
	time.Sleep(time.Second * 5)

}

// Fails with error since ResponseRecorder does not implment Hijackable interface
func TestBlipEndpointResponseRecorder(t *testing.T) {

	var rt RestTester
	defer rt.Close()

	// rt.Bucket()
	serverContext := rt.ServerContext()
	dbContext := serverContext.Database("db")
	log.Printf("dbContext: %v", dbContext)

	request, err := http.NewRequest("GET", "http://localhost/db/_blipsync", bytes.NewBufferString(""))
	request.RequestURI = "db/_blipsync" // This doesn't get filled in by NewRequest
	FixQuotedSlashes(request)
	if err != nil {
		panic(fmt.Sprintf("http.NewRequest failed: %v", err))
	}
	response := &TestResponse{httptest.NewRecorder(), request}
	response.Code = 200 // doesn't seem to be initialized by default; filed Go bug #4188
	rt.TestPublicHandler().ServeHTTP(response, request)

	response.DumpBody()

	// blipSyncHandler := makeHandler(serverContext, adminPrivs, (*handler).handleBLIPSync)

	// publicHandler := CreatePublicHandler(serverContext)

	// srv := httptest.NewServer(publicHandler)

	// destUrl := fmt.Sprintf("%s/todo/_blipsync", srv.URL)
	// destUrl := fmt.Sprintf("%s", srv.URL)

	// u, _ := url.Parse(destUrl)

	// u.Scheme = "ws"

	//context := blip.NewContext()
	//context.LogMessages = true
	//context.LogFrames = true
	//origin := "http://localhost" // TODO: what should be used here?
	//sender, err := context.Dial(u.String(), origin)
	//if err != nil {
	//	panic("Error opening WebSocket: " + err.Error())
	//}

	//log.Printf("sender: %v", sender)

	//request := blip.NewRequest()
	//request.SetProfile("/db/_blipsync")
	//request.Properties["Content-Type"] = "application/octet-stream"
	//body := make([]byte, rand.Intn(100))
	//for i := 0; i < len(body); i++ {
	//	body[i] = byte(i % 256)
	//}
	//request.SetBody(body)
	//sender.Send(request)
	//
	//log.Printf("sent request: %v", request)

	//response := rt.SendRequest("GET", "/db/doc", `{"prop":true}`)

	// TODO: send websocket upgrade request

}
