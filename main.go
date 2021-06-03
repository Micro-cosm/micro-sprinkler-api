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
	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	returnSet                  = "{}"
	pollDuration time.Duration = 8
	debug        bool          = false
)

func pod1(w http.ResponseWriter, r *http.Request) {
	device, ok := r.URL.Query()["device"]
	referer := r.Header.Get("referer")
	pod1Mac := "E8:DB:84:97:BA:85"
	pod2Mac := "8C:AA:B5:C6:42:A6"

	if ok &&
		device[0] == referer &&
		(pod1Mac == referer || pod2Mac == referer) {

		returnSet = "{}"
		if debug {
			log.Printf("Subscription poll request from pod: #1\tdevice: %s", device[0])
		}
		if err := pullMsgsSync("1"); err != nil {
			return
		}
		if debug {
			log.Printf("\tListener timeout reached, returning: %s\tto: %s", returnSet, referer)
		}
		log.Printf("\tReturning: %s To: %s", returnSet, referer)
		fmt.Fprintf(w, returnSet)
	} else {
		log.Println("No valid deviceId was provided... ignoring request.")
		msg := "{\"msg\":\"Sorry, can't help you :(\"}"
		fmt.Fprintf(w, "%s", msg)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	msg := "{\"msg\":\"Howdy!\"}"
	fmt.Fprintf(w, "%s", msg)
}

func main() {
	debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))
	routeBase := os.Getenv("ROUTE_BASE")
	log.Println("Sparking up server on port: 8080(possibly mapped)", routeBase)
	// http.HandleFunc( routeBase + "1", pod1 )
	// http.HandleFunc( routeBase, index )
	// http.HandleFunc( "/api/pod/2", pod2 )
	var router = mux.NewRouter()
	router.Use(commonMiddleware)

	router.HandleFunc(routeBase+"1", pod1).Methods("GET")
	router.HandleFunc(routeBase, index).Methods("GET")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func pullMsgsSync(podNo string) error {
	projectId := "weja-us"
	subID := "pod" + podNo
	ctx := context.Background()

	if debug {
		log.Printf("\t\tchecking subscription pod1...")
	}

	client, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	sub := client.Subscription(subID)
	sub.ReceiveSettings.Synchronous = true // Synchronous mode uses Pull RPC rather than StreamingPull RPC, which guarantees MaxOutstandingMessages
	sub.ReceiveSettings.MaxOutstandingMessages = 1

	log.Printf("Subscription request: pod%s", podNo)

	ctx, cancel := context.WithTimeout(ctx, pollDuration*time.Second) // Receive messages
	defer cancel()
	cm := make(chan *pubsub.Message) // Create a channel for incoming messages
	defer close(cm)

	go func() { // goroutine for processing messages individually
		if debug {
			log.Printf("\tIterating published instructions...")
		}
		count := 0
		for msg := range cm {
			count++
			log.Printf("\t\trecord: %d", count)
			msg.Ack()
			log.Printf("\t\t\tack'd record: %d", count)
			returnSet = string(msg.Data)
		}
	}()
	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) { cm <- msg }) // Receive blocks until the passed in context is done.
	if err != nil && status.Code(err) != codes.Canceled {
		return fmt.Errorf("receive %v", err)
	}

	return nil
}
