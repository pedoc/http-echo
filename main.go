package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/hashicorp/http-echo/version"
)

var (
	listenFlag  = flag.String("listen", ":5678", "address and port to listen")
	textFlag    = flag.String("text", "", "text to put on the webpage")
	versionFlag = flag.Bool("version", false, "display version information")

	// stdoutW and stderrW are for overriding in test.
	stdoutW = os.Stdout
	stderrW = os.Stderr
)

func main() {
	flag.Parse()

	// Asking for the version?
	if *versionFlag {
		fmt.Fprintln(stderrW, version.HumanVersion)
		os.Exit(0)
	}

	// Validation
	// if *textFlag == "" {
	// 	fmt.Fprintln(stderrW, "Missing -text option!")
	// 	os.Exit(127)
	// }

	args := flag.Args()
	if len(args) > 0 {
		fmt.Fprintln(stderrW, "Too many arguments!")
		os.Exit(127)
	}

	// Flag gets printed as a page
	mux := http.NewServeMux()
	mux.HandleFunc("/", httpLog(stdoutW, withAppHeaders(httpEcho(*textFlag))))

	// Health endpoint
	mux.HandleFunc("/health", withAppHeaders(httpHealth()))

	server := &http.Server{
		Addr:    *listenFlag,
		Handler: mux,
	}
	serverCh := make(chan struct{})
	go func() {
		log.Printf("[INFO] server is listening on %s\n", *listenFlag)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[ERR] server exited with: %s", err)
		}
		close(serverCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	// Wait for interrupt
	<-signalCh

	log.Printf("[INFO] received interrupt, shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("[ERR] failed to shutdown server: %s", err)
	}

	// If we got this far, it was an interrupt, so don't exit cleanly
	os.Exit(2)
}

func httpEcho(v string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if *textFlag != "" {
			fmt.Fprintln(w, v)
		} else {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			clientInfo := fmt.Sprintf("From: %s\n\n", r.RemoteAddr)
			fmt.Fprint(w, clientInfo)
			fmt.Print(clientInfo)

			requestInfo := fmt.Sprintf("[%s] [%s://%s%s]\n\n", r.Method, scheme, r.Host, r.RequestURI)
			fmt.Fprint(w, requestInfo)
			fmt.Print(requestInfo)

			fmt.Fprintln(w, "[Headers]")
			fmt.Println("[Headers]")
			names := make([]string, 0, len(r.Header))
			for name := range r.Header {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				for _, value := range r.Header[name] {
					headerInfo := fmt.Sprintf("%s: %s\n", name, value)
					fmt.Fprint(w, headerInfo)
					fmt.Print(headerInfo)
				}
			}

			fmt.Fprintln(w, "\n[Body]")
			fmt.Println("\n[Body]")
			if r.Body != nil {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					errorInfo := fmt.Sprintf("Read body error: %v\n", err)
					fmt.Fprint(w, errorInfo)
					fmt.Print(errorInfo)
				} else {
					bodyInfo := string(body) + "\n"
					fmt.Fprint(w, bodyInfo)
					fmt.Print(bodyInfo)
				}
			}
		}
	}
}

func httpHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"status":"ok"}`)
	}
}
