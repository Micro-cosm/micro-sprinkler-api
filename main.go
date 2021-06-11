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
	debug     = false
	returnSet = "{}"
	safeList  = [4]string{
		"00:00:00:00:00:00",
		"E8:DB:84:97:BA:85",
		"8C:AA:B5:C6:42:A6",
		"EC:FA:BC:C0:AB:B1",
	}

	pollDuration time.Duration = 8
)

func pullMsgsSync(podNo string) error {

	projectId := "weja-us"
	subID := "pod" + podNo
	ctx := context.Background()
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
	sub.ReceiveSettings.Synchronous = true // Pull, in place of StreamingPull, will respect MaxOutstandingMessages
	sub.ReceiveSettings.MaxOutstandingMessages = 1

	log.Printf("Subscription request: pod%s", podNo)

	ctx, cancel := context.WithTimeout(ctx, pollDuration*time.Second) // Receive messages
	defer cancel()
	cm := make(chan *pubsub.Message) // Channel for incoming messages
	defer close(cm)

	go func() { // goroutine iterates against next un-ack'd message
		log.Printf("\tprocessing unread messsages...")
		count := 0
		for msg := range cm {
			count++
			msg.Ack()
			log.Printf("\t\tmessage #%d... ack'd", count)
			returnSet = string(msg.Data)
		}
	}()

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) { cm <- msg }) // Receive blocks until context is exhausted
	if err != nil && status.Code(err) != codes.Canceled {
		return fmt.Errorf("receive %v", err)
	}

	return nil
}

func slContains(device string) bool {
	for _, s := range safeList {
		if s == device {
			return true
		}
	}
	return false
}

// Todo: Replace pod1 & pod2 with polling support for 1..n pod(s)
func pod1(w http.ResponseWriter, r *http.Request) {

	device, ok := r.URL.Query()["device"]
	referer := r.Header.Get("referer")

	if ok && slContains(device[0]) && device[0] == referer {

		returnSet = "{}"
		if err := pullMsgsSync("1"); err != nil {
			return
		}

		log.Printf("\ttarget device: %s\n\t\t\ttarget intruction set:\n%s\n", referer, returnSet)
		fprintf, err := fmt.Fprintf(w, returnSet)
		if err != nil {
			log.Println(fprintf)
		} // DETERMINE IF "return" SHOULD BE EXPLICIT HERE
	} else {

		log.Println("invalid device id in request... ignored.")
		for name, values := range r.Header {
			for _, value := range values {
				log.Println(name, value)
			}
		} // Log all headers for invalid requests
		msg := "{\"msg\":\"Sorry, can't help you :(\"}"
		fprintf, err := fmt.Fprintf(w, "%s", msg)
		if err != nil {
			log.Println(fprintf)
		} // DETERMINE IF "return" SHOULD BE EXPLICIT HERE
	}
}

func pod2(w http.ResponseWriter, r *http.Request) {

	device, ok := r.URL.Query()["device"]
	referer := r.Header.Get("referer")

	if ok && slContains(device[0]) && device[0] == referer {

		returnSet = "{}"
		if err := pullMsgsSync("2"); err != nil {
			return
		}

		log.Printf("\ttarget device: %s\ttarget intruction set:\n%s", referer, returnSet)
		fprintf, err := fmt.Fprintf(w, returnSet)
		if err != nil {
			log.Println(fprintf)
		} // DETERMINE IF "return" SHOULD BE EXPLICIT HERE
	} else {

		log.Println("invalid device id in request... ignored.")
		for name, values := range r.Header {
			for _, value := range values {
				log.Println(name, value)
			}
		} // Log all headers for invalid requests
		msg := "{\"msg\":\"Sorry, can't help you :(\"}"
		fprintf, err := fmt.Fprintf(w, "%s", msg)
		if err != nil {
			log.Println(fprintf)
		} // DETERMINE IF "return" SHOULD BE EXPLICIT HERE
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	msg := "{\"msg\":\"Howdy!\"}"
	fprintf, err := fmt.Fprintf(w, "%s", msg)
	if err != nil {
		log.Println(fprintf, r)
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
