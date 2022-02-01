package faucet

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/guyu96/go-timbre/faucet/limiter"
	"github.com/guyu96/go-timbre/net"
)

const (
	limiterTick       = 20 * time.Second
	limiterExpiryTime = 15 * time.Second
	amountToSend      = 1000
)

func formHandler(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}
	fmt.Fprintf(w, "POST request successful")

	address := r.FormValue("add")
	if address == "" {
		http.Error(w, "address must be specified to send decibels to", 400)
		return
	}
	fmt.Println("Requesting to send decibels to: ", address)

}

//RunFaucet runs the faucet
func RunFaucet(node *net.Node) {
	fmt.Println("Starting the faucet...")

	accessLimiter := limiter.NewLimiter(limiterExpiryTime)
	go func() { //Will periodically clean entries which are not banned
		c := time.Tick(limiterTick)
		for range c {
			accessLimiter.RemoveAllowedEntries()
		}
	}()

	fileServer := http.FileServer(http.Dir("../../faucet")) //will serve the index html
	http.Handle("/", fileServer)

	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {

		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}
		// fmt.Fprintf(w, "POST request successful")

		address := r.FormValue("add")
		if address == "" {
			http.Error(w, "address must be specified to send decibels to", 400)
			return
		}
		fmt.Println("Requesting to send decibels to: ", address)

		//Checking if address is banned for the expiry time
		t, allowed := accessLimiter.AllowedToGetFunds(address)
		// fmt.Println("allowed: ", allowed, t)
		if !allowed {
			fmt.Println("Access limit has already been reached. Try later")
			w.Header().Add("Retry-After", fmt.Sprintf("%f", t.Seconds()))
			http.Error(w, fmt.Sprintf("Too Many Requests, please wait %f sec", t.Seconds()), http.StatusTooManyRequests)
			return
		}

		fmt.Println("sending transaction")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := node.Services.DoAmountTrans(ctx, address, amountToSend)

		if err != nil {
			fmt.Println("failed to Post request: ", err.Error())
			http.Error(w, err.Error(), 500)
			return
		}

		accessLimiter.Add(address)

		fmt.Println("Request successful", address)
		// w.Header().Add("Message-Cid", address))
		w.WriteHeader(200)
		fmt.Fprint(w, "Successfully sent transaction! ") // nolint: errcheck
		// fmt.Fprintln(w, msgcid.String())        // nolint: errcheck

	})

	// http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Fprintf(w, "Hello!")
	// })

	fmt.Printf("Starting server at port 9090\n")
	if err := http.ListenAndServe("127.0.0.1:9090", nil); err != nil {
		log.Fatal(err)
	}
}
