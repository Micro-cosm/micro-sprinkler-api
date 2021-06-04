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
	safeList                   = [2]string{"E8:DB:84:97:BA:85", "8C:AA:B5:C6:42:A6"}
)

func slContains(device string) bool {
	for _, s := range safeList {
		if s == device {
			return true
		}
	}
	return false
}

func pod1(w http.ResponseWriter, r *http.Request) {

	device, ok := r.URL.Query()["device"]
	referer := r.Header.Get("referer")

	if ok && slContains(device[0]) && device[0] == referer {

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
		fprintf, err := fmt.Fprintf(w, returnSet)
		if err != nil {
			log.Println(fprintf)
			return
		}

	} else {

		log.Println("No valid deviceId was provided... ignoring request.")
		msg := "{\"msg\":\"Sorry, can't help you :(\"}"
		fprintf, err := fmt.Fprintf(w, "%s", msg)
		if err != nil {
			log.Println(fprintf)
			return
		}
	}
}

// Todo: Replace individual pod subscription polling with a generic polling routine with support for 1..n pods

func pod2(w http.ResponseWriter, r *http.Request) {

	device, ok := r.URL.Query()["device"]
	referer := r.Header.Get("referer")

	if ok && slContains(device[0]) && device[0] == referer {

		returnSet = "{}"
		if debug {
			log.Printf("Subscription poll request from pod: #2\tdevice: %s", device[0])
		}

		if err := pullMsgsSync("2"); err != nil {
			return
		}

		if debug {
			log.Printf("\tListener timeout reached, returning: %s\tto: %s", returnSet, referer)
		}
		log.Printf("\tReturning: %s To: %s", returnSet, referer)
		fprintf, err := fmt.Fprintf(w, returnSet)
		if err != nil {
			log.Println(fprintf)
			return
		}

	} else {

		log.Println("No valid deviceId was provided... ignoring request.")
		msg := "{\"msg\":\"Sorry, can't help you :(\"}"
		fprintf, err := fmt.Fprintf(w, "%s", msg)
		if err != nil {
			log.Println(fprintf)
			return
		}
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	msg := "{\"msg\":\"Howdy!\"}"
	fprintf, err := fmt.Fprintf(w, "%s", msg)
	if err != nil {
		log.Println(fprintf, r)
		return
	}
}

func main() {

	debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))
	routeBase := os.Getenv("ROUTE_BASE")
	log.Println("Sparking up server on port: 8080(possibly mapped)", routeBase)
	var router = mux.NewRouter()
	router.Use(commonMiddleware)

	router.HandleFunc(routeBase+"1", pod1).Methods("GET")
	router.HandleFunc(routeBase+"2", pod2).Methods("GET")
	router.HandleFunc(routeBase, index).Methods("GET")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		},
	)
}

func pullMsgsSync(podNo string) error {

	projectId := "weja-us"
	subID := "pod" + podNo
	ctx := context.Background()

	if debug {
		log.Printf("\t\tchecking subscription pod%s...", podNo)
	}

	client, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer func(client *pubsub.Client) {
		err := client.Close()
		if err != nil {
			log.Println(err)
		}
	}(client)

	sub := client.Subscription(subID)
	sub.ReceiveSettings.Synchronous = true // Synchronous mode uses Pull RPC rather than StreamingPull RPC, should guarantee MaxOutstandingMessages
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
