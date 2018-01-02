package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/couchbase/go-blip"
	"github.com/couchbase/sync_gateway/base"
	"github.com/couchbaselabs/go.assert"
)

// What's missing:
// - Getting and setting a checkpoint
// - Getting and setting attachments
// - Continuous changes feed
// - No-conflicts mode replication (proposeChanges)
// - Unsolicited rev request
// - Connect to public port with authentication

// Make sure it's not possible to have two outstanding subChanges w/ continuous=true.
func TestConcurrentChangesSubscriptions(t *testing.T) {


}

// Start subChanges w/ continuous=true, batchsize=20
// Make several updates
// Wait until we get the expected updates
func TestContinousChangesSubscription(t *testing.T) {


}

func TestMultiChannelContinousChangesSubscription(t *testing.T) {


}


// This test performs the following steps against the Sync Gateway passive blip replicator:
//
// - Setup
//   - Create an httptest server listening on a port that wraps the Sync Gateway Admin Handler
//   - Make a BLIP/Websocket client connection to Sync Gateway
// - Test
//   - Verify Sync Gateway will accept the doc revision that is about to be sent
//   - Send the doc revision in a rev request
//   - Call changes endpoint and verify that it knows about the revision just sent
//   - Call subChanges api and make sure we get expected changes back
func TestBlipPushRevisionInspectChanges(t *testing.T) {

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

	// Verify Sync Gateway will accept the doc revision that is about to be sent
	var changeList [][]interface{}
	changesRequest := blip.NewRequest()
	changesRequest.SetProfile("changes")  // TODO: make a constant for "changes" and use it everywhere
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
	assert.True(t, len(changeList) == 1) // Should be 1 row, corresponding to the single doc that was queried in changes
	changeRow := changeList[0]
	assert.True(t, len(changeRow) == 0) // Should be empty, meaning the server is saying it doesn't have the revision yet

	// Send the doc revision in a rev request
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

	// Call changes with a hypothetical new revision, assert that it returns last pushed revision
	var changeList2 [][]interface{}
	changesRequest2 := blip.NewRequest()
	changesRequest2.SetProfile("changes")
	changesRequest2.SetBody([]byte(`[["2", "foo", "2-xyz", false]]`)) // [sequence, docID, revID]
	sent2 := sender.Send(changesRequest2)
	assert.True(t, sent2)
	changesResponse2 := changesRequest2.Response()
	assert.Equals(t, changesResponse2.SerialNumber(), changesRequest2.SerialNumber())
	body2, err := changesResponse2.Body()
	assertNoError(t, err, "Error reading changes response body")
	log.Printf("changes2 response body: %s", body2)
	err = json.Unmarshal(body2, &changeList2)
	assertNoError(t, err, "Error unmarshalling response body")
	assert.True(t, len(changeList2) == 1) // Should be 1 row, corresponding to the single doc that was queried in changes
	changeRow2 := changeList2[0]
	assert.True(t, len(changeRow2) == 1) // Should have 1 item in row, which is the rev id of the previous revision pushed
	assert.Equals(t, changeRow2[0], "1-abc")

	// Call subChanges api and make sure we get expected changes back
	receviedChangesRequestWg := sync.WaitGroup{}
	// When this test sends subChanges, Sync Gateway will send a changes request that must be handled
	blipContext.HandlerForProfile["changes"] = func(request *blip.Message) {
		log.Printf("got changes message: %+v", request)
		body, err := request.Body()
		log.Printf("changes body: %v, err: %v", string(body), err)
		// Expected changes body: [[1,"foo","1-abc"]]
		changeListReceived := [][]interface{}{}
		err = json.Unmarshal(body, &changeListReceived)
		assertNoError(t, err, "Error unmarshalling changes recevied")
		assert.True(t, len(changeListReceived) == 1)
		change := changeListReceived[0] // [1,"foo","1-abc"]
		assert.True(t, len(change) == 3)
		assert.Equals(t, change[0].(float64), float64(1)) // Original sequence sent in pushed rev
		assert.Equals(t, change[1], "foo")                // Doc id of pushed rev
		assert.Equals(t, change[2], "1-abc")              // Rev id of pushed rev
		receviedChangesRequestWg.Done()

		// TODO: send a reply saying we don't need any changes

	}
	subChangesRequest := blip.NewRequest()
	subChangesRequest.SetProfile("subChanges")
	subChangesRequest.Properties["continuous"] = "true"

	sent = sender.Send(subChangesRequest)
	assert.True(t, sent)
	receviedChangesRequestWg.Add(1)
	subChangesResponse := subChangesRequest.Response()
	assert.Equals(t, subChangesResponse.SerialNumber(), subChangesRequest.SerialNumber())

	// Wait until we got the expected incoming changes request
	receviedChangesRequestWg.Wait()


	// changes := getChangesSince() ....
	

}

// Fails with error since ResponseRecorder does not implment Hijackable interface
func DisabledTestBlipEndpointResponseRecorder(t *testing.T) {

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

}
