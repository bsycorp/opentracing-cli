package main

import (
	"encoding/json"
	"flag"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"io/ioutil"
	"log"
	"math/rand"
	"time"
)

type SpanState struct {
	Env string
	Service string
	Resource string
	Operation string
	StartMillis time.Time
	SpanID uint64
	Tags map[string]string
	Context tracer.TextMapCarrier
	ParentContext tracer.TextMapCarrier
}

func main() {
	actionPtr := flag.String("action", "", "'start' or 'finish'")
	envPtr := flag.String("env", "unknown-env", "The env name visible for the span")
	servicePtr := flag.String("service", "unknown-service", "The service name visible for the span")
	resourcePtr := flag.String("resource", "unknown-resource", "The resource name visible for the span")
	operationPtr := flag.String("operation", "unknown-operation", "The operation name visible for the span")
	currentSpanStatePtr := flag.String("state", "", "The file path to store/retrieve the started span state")
	parentSpanStatePtr := flag.String("parent", "", "The file path to store/retrieve the parent span state")
	tagsJsonPtr := flag.String("tags", "{}", "The extra tags to add to the span, as JSON")
	epochTimeMillisPtr := flag.Int64("epoch-time", -1, "The time the span started / finished, default to time.Now()")
	isoTimeMillisPtr := flag.String("iso-time", "", "The time the span started / finished, default to time.Now()")
	flag.Parse()

	//default start time to now if omitted
	var err error = nil
	actionTime := time.Now()
	if *epochTimeMillisPtr > 0 {
		actionTime = time.Unix(0, *epochTimeMillisPtr* int64(time.Millisecond))
		log.Printf("overriding action time to %s", actionTime)
	}
	if len(*isoTimeMillisPtr) > 0 {
		actionTime, err = time.Parse(time.RFC3339, *isoTimeMillisPtr)
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Printf("overriding action time to %s", actionTime)
	}

	if string(*actionPtr) == "start" {
		start(actionTime, *envPtr, *servicePtr, *resourcePtr, *operationPtr, *tagsJsonPtr, *currentSpanStatePtr, *parentSpanStatePtr)
	} else if *actionPtr == "finish" {
		finish(actionTime, *currentSpanStatePtr)
	} else {
		log.Fatal("unsupported action, should be 'start' or 'finish'")
	}

}

func start(startTime time.Time, env string, service string, resource string, operation string, tagsJson string, currentStateFilePath string, parentStateFilePath string) {
	if len(currentStateFilePath) == 0 {
		log.Fatal("no state path found, set via -state")
	}

	tracer.Start(tracer.WithServiceName(service))

	//dont love this but should be ok
	rand.Seed(startTime.UnixNano())
	spanID := rand.Uint64()

	tags := &map[string]string{}
	err := json.Unmarshal([]byte(tagsJson), tags)
	if err != nil {
		log.Fatal(err.Error())
	}

	var span ddtrace.Span = nil
	var parentSpanContextCarrier tracer.TextMapCarrier = nil
	if len(parentStateFilePath) == 0 {
		span = tracer.StartSpan(
			operation,
			tracer.WithSpanID(spanID),
			tracer.ResourceName(resource),
			tracer.Tag("Env", env),
			tracer.StartTime(startTime))
	} else {
		var parentSpanState *SpanState = nil
		parentContextJson, err := ioutil.ReadFile(parentStateFilePath)
		if err != nil {
			log.Fatal(err.Error())
		}
		parentSpanState = &SpanState{}
		err = json.Unmarshal(parentContextJson, parentSpanState)
		if err != nil {
			log.Fatal(err.Error())
		}

		parentSpanContextCarrier = parentSpanState.Context
		parentSpanContext, err := tracer.Extract(parentSpanContextCarrier)
		if err != nil {
			log.Fatal(err.Error())
		}

		span = tracer.StartSpan(
			operation,
			tracer.ChildOf(parentSpanContext),
			tracer.WithSpanID(spanID),
			tracer.ResourceName(resource),
			tracer.Tag("Env", env),
			tracer.StartTime(startTime))
	}

	//serialise span to file
	carrier := tracer.TextMapCarrier{}
	err = tracer.Inject(span.Context(), carrier)
	if err != nil {
		log.Fatal(err.Error())
	}
	contextJson, err := json.Marshal(&SpanState{env, service, resource, operation, startTime, spanID,*tags, carrier, parentSpanContextCarrier})
	if err != nil {
		log.Fatal(err.Error())
	}
	//fmt.Printf("Writing span state to '%s'", currentStateFilePath)
	err = ioutil.WriteFile(currentStateFilePath, contextJson, 0644)
	if err != nil {
		log.Fatal(err.Error())
	}
	//fmt.Println(string(contextJson))
}

func finish(finishTime time.Time, currentStateFilePath string) {
	currentContextJson, err := ioutil.ReadFile(currentStateFilePath)
	if err != nil {
		log.Fatal(err.Error())
	}

	currentSpanState := &SpanState{}
	err = json.Unmarshal(currentContextJson, currentSpanState)
	if err != nil {
		log.Fatal(err.Error())
	}

	//start tracer, add global tags if found
	startOptions := []tracer.StartOption { tracer.WithServiceName(currentSpanState.Service), tracer.WithEnv(currentSpanState.Env) }
	for k, v := range currentSpanState.Tags {
		startOptions = append(startOptions, tracer.WithGlobalTag(k, v))
	}
	tracer.Start(startOptions...)
	defer tracer.Stop()

	var span ddtrace.Span = nil
	//if we have a parent span then add it to span declaration, duplication here sucks but cbf optimising
	if currentSpanState.ParentContext == nil {
		span = tracer.StartSpan(
			currentSpanState.Operation,
			tracer.WithSpanID(currentSpanState.SpanID),
			tracer.ResourceName(currentSpanState.Resource),
			tracer.StartTime(currentSpanState.StartMillis))

		//fmt.Printf("Finished span with id: %s", span.Context().SpanID())

	} else {
		parentSpanContext, err := tracer.Extract(currentSpanState.ParentContext)
		if err != nil {
			log.Fatal(err.Error())
		}

		span = tracer.StartSpan(
			currentSpanState.Operation,
			tracer.ChildOf(parentSpanContext),
			tracer.WithSpanID(currentSpanState.SpanID),
			tracer.ResourceName(currentSpanState.Resource),
			tracer.StartTime(currentSpanState.StartMillis))
		//fmt.Printf("Finished span with id: %s parent: %s", span.Context().SpanID(), parentSpanContext.SpanID())
	}

	span.Finish(tracer.FinishTime(finishTime))
}
