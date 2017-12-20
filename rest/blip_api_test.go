package rest

import (
	"log"
	"net/http/httptest"
	"testing"

	"bytes"
	"fmt"
	"net/http"
)

func TestBlipEndpoint(t *testing.T) {

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
