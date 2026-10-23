// Command probestub is the provider a probe integration leg asks: it answers
// the SystemOne decision surface and refuses every other path as a chat
// server refuses a decision model, so the delivered binary is shown to reach
// the surface its pool entry declares rather than the chat path.
//
// It prints the base URL it listens on as its first line and serves until it
// is killed. It reads no environment, no file and no upstream: the leg that
// runs it is hermetic.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "probestub:", err)
		os.Exit(1)
	}
	fmt.Println("http://" + ln.Addr().String())
	if err := http.Serve(ln, handler()); err != nil {
		fmt.Fprintln(os.Stderr, "probestub:", err)
		os.Exit(1)
	}
}

// handler answers the probe's one decision question and refuses the chat path
// in the words a provider refuses a decision model, so a probe that took Chat
// fails instead of passing on a chat reply.
func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/systemone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"stub","answers":{"probe":{"type":"noul","noul":0.96}},`+
			`"usage":{"input_tokens":12,"output_tokens":1}}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"this model is not an LLM / chat model"}}`)
	})
	return mux
}
