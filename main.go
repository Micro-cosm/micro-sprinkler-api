

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	returnSet = "{}"
	pollDuration time.Duration	= 8
	debug bool = false
)


func pod1( w http.ResponseWriter, r *http.Request ) {
	device, ok	:= r.URL.Query()["device"]
	referer		:= r.Header.Get("referer")
	pod1		:= "E8:DB:84:97:BA:85"

	if ok && device[0] == referer && pod1 == referer {
		returnSet = "{}"
		if (debug) { log.Printf("Subscription poll request from pod: #1\tdevice: %s", device[0]) }
		if err := pullMsgsSync("1"); err != nil { return }
		if (debug) { log.Printf("\tListener timeout reached, returning: %s\tto: %s", returnSet, referer )}
		log.Printf("\tReturning: %s To: %s", returnSet, referer)
		fmt.Fprintf(w, returnSet)
	} else {
		log.Println("No valid deviceId was provided... ignoring request.")
		fmt.Fprintf(w, "Sorry, can't help you :(")
	}
}


func pod2( w http.ResponseWriter, r *http.Request, ) {
	device, ok	:= r.URL.Query()["device"]
	referer		:= r.Header.Get("referer")
	pod2		:= ""

	if ok && device[0] == referer && pod2 == referer {
		returnSet = "{}"
		if (debug) { log.Printf("Subscription poll request from pod: #2\tdevice: %s", device[0]) }
		if err := pullMsgsSync("2"); err != nil { return }
		if (debug) { log.Printf("\tListener timeout reached, returning: %s\tto: %s", returnSet, referer )}
		log.Printf("\tReturning: %s To: %s", returnSet, referer)
		fmt.Fprintf(w, returnSet)
	} else {
		log.Println("No valid deviceId was provided... ignoring request.")
		fmt.Fprintf(w, "Sorry, can't help you :(")
	}
}


func main() {
	port	:= os.Getenv("REMOTE_PORT")
	debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))

	log.Printf("Sparking up server on port: %s", port)

	http.HandleFunc("/api/pod/1", pod1)
	http.HandleFunc("/api/pod/2", pod2)

	if err := http.ListenAndServe(":" + port, nil);	err != nil { log.Fatal(err) }
}


func pullMsgsSync( podNo string ) error {
	projectId	:= "weja-us"
	subID		:= "pod" + podNo
	ctx			:= context.Background()

	if (debug) { log.Printf("\t\tchecking subscription pod1...") }

	client, err	:= pubsub.NewClient(ctx, projectId)
	if err != nil { return fmt.Errorf("pubsub.NewClient: %v", err)}
	defer client.Close()

	sub := client.Subscription(subID)
	sub.ReceiveSettings.Synchronous = true									// Synchronous mode uses Pull RPC rather than StreamingPull RPC, which guarantees MaxOutstandingMessages
	sub.ReceiveSettings.MaxOutstandingMessages = 1

	log.Printf("Subscription request: pod%s", podNo)

	ctx, cancel	:= context.WithTimeout(ctx, pollDuration * time.Second) 	// Receive messages
	defer cancel()
	cm			:= make(chan *pubsub.Message)								// Create a channel for incoming messages
	defer close(cm)

	go func() {																// goroutine for processing messages individually
		if debug { log.Printf("\tIterating published instructions...") }
		count := 0
		for msg	:= range cm {
			count++
			log.Printf("\t\trecord: %d", count)
			msg.Ack()
			log.Printf("\t\t\tack'd record: %d", count)
			returnSet	= string(msg.Data)
		}
	}()
	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) { cm <- msg })	// Receive blocks until the passed in context is done.
	if err != nil && status.Code(err) != codes.Canceled { return fmt.Errorf("Receive: %v", err) }

	return nil
}
