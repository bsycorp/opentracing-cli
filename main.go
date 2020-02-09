package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"io/ioutil"
	"math/rand"
	"time"
)

type SpanState struct {
	Env string
	Service string
	Operation string
	StartMillis time.Time
	SpanID uint64
	Context tracer.TextMapCarrier
}

func main() {
	actionPtr := flag.String("action", "", "'start' or 'finish'")
	envPtr := flag.String("env", "", "The env name visible for the span")
	servicePtr := flag.String("service", "", "The service name visible for the span")
	operationPtr := flag.String("operation", "", "The operation name visible for the span")
	currentSpanStatePtr := flag.String("state", "", "The file path to store/retrieve the started span state")
	parentSpanStatePtr := flag.String("parent", "", "The file path to store/retrieve the parent span state")
	flag.Parse()

	if string(*actionPtr) == "start" {
		start(string(*envPtr), string(*servicePtr), string(*operationPtr), string(*currentSpanStatePtr), string(*parentSpanStatePtr))
	} else if string(*actionPtr) == "finish" {
		finish(string(*currentSpanStatePtr), string(*parentSpanStatePtr))
	} else {
		fmt.Errorf("Unsupported action")
	}

}

func start(env string, service string, operation string, currentStateFilePath string, parentStateFilePath string) {
	var parentSpanState *SpanState = nil
	if(len(parentStateFilePath) > 0) {
		parentContextJson, err := ioutil.ReadFile(parentStateFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		parentSpanState = &SpanState{}
		err = json.Unmarshal(parentContextJson, parentSpanState)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	tracer.Start(tracer.WithServiceName(service))

	rand.Seed(time.Now().UnixNano())
	spanID := rand.Uint64()

	var span ddtrace.Span = nil
	if(len(parentStateFilePath) == 0) {
		span = tracer.StartSpan(
			operation,
			tracer.WithSpanID(spanID),
			tracer.Tag("Env", env),
			tracer.StartTime(time.Now()))
	} else {
		parentSpanContext, err := tracer.Extract(parentSpanState.Context)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		span = tracer.StartSpan(
			operation,
			tracer.ChildOf(parentSpanContext),
			tracer.WithSpanID(spanID),
			tracer.Tag("Env", env),
			tracer.StartTime(time.Now()))
	}

	//serialise span to file
	carrier := tracer.TextMapCarrier{}
	err := tracer.Inject(span.Context(), carrier)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	contextJson, err := json.Marshal(&SpanState{env, service, operation, time.Now(), spanID, carrier})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	//fmt.Printf("Writing span state to '%s'", currentStateFilePath)
	err = ioutil.WriteFile(currentStateFilePath, contextJson, 0644)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(contextJson))
}

func finish(currentStateFilePath string, parentStateFilePath string) {
	currentContextJson, err := ioutil.ReadFile(currentStateFilePath)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	currentSpanState := &SpanState{}
	err = json.Unmarshal(currentContextJson, currentSpanState)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	tracer.Start(tracer.WithServiceName(currentSpanState.Service))
	defer tracer.Stop()

	var span ddtrace.Span = nil
	//if we have a parent span then add it to span declaration, duplication here sucks but cbf optimising
	if(len(parentStateFilePath) == 0) {
		span = tracer.StartSpan(
			currentSpanState.Operation,
			tracer.WithSpanID(currentSpanState.SpanID),
			tracer.Tag("Env", currentSpanState.Env),
			tracer.StartTime(currentSpanState.StartMillis))

		fmt.Printf("Finished span with id: %s", span.Context().SpanID())
	} else {
		parentContextJson, err := ioutil.ReadFile(parentStateFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		parentSpanState := &SpanState{}
		err = json.Unmarshal(parentContextJson, parentSpanState)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		parentSpanContext, err := tracer.Extract(parentSpanState.Context)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		span = tracer.StartSpan(
			currentSpanState.Operation,
			tracer.ChildOf(parentSpanContext),
			tracer.WithSpanID(currentSpanState.SpanID),
			tracer.Tag("Env", currentSpanState.Env),
			tracer.StartTime(currentSpanState.StartMillis))
		fmt.Printf("Finished span with id: %s parent: %s", span.Context().SpanID(), parentSpanContext.SpanID())
	}

	span.Finish()
}
