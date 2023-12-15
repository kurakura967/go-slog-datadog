package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

func main() {

	tracer.Start(
		tracer.WithService("sample-service"),
		tracer.WithEnv("dev"),
		// tracer.WithAgentAddr("datadog-agent:8126"),
		tracer.WithDebugMode(true),
	)
	defer tracer.Stop()

	// Create a traced mux router
	mux := httptrace.NewServeMux()
	// Continue using the router as you normally would.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		span, ctx := tracer.StartSpanFromContext(r.Context(), "GetHelloWorldHandler")
		defer span.Finish()

		MyFunc(ctx)

		w.Write([]byte("Hello World!!!\n"))
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func MyFunc(ctx context.Context) {
	span, _ := tracer.StartSpanFromContext(ctx, "MyFunc")
	defer span.Finish()
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	datadogHandler := NewDatadogHandler(jsonHandler)
	logger := slog.New(datadogHandler)
	logger.InfoContext(ctx, "logger from MyFunc")
}
