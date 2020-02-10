This is a CLI tool to support emitting opentracing spans from the command line, this can be useful for instrumenting scripts or other processes that aren't strictly "applications". At the moment it only supports Datadog but could be extended to add other providers if desired, i'm unsure what the effort would be and what features would be supported but likely possible.

The cli should be used in a two step `start` and `finish` fashion, because you likely want to put the start at the top of your script and the finish at the end, there isn't the more traditional single wrapping call like more standard APM span reporters. Because of this two steps, all the context of the span is captured in the first call and written to a state file that is then referenced in the second finish call that actually emits the span.
Parent/child relationships can be established by referencing a parent span state file from the child.


To start regular span
`opentracing-cli -action start -env env -service service -resource resource -operation operation -state state.json`

To start child span
`opentracing-cli -action start -env env -service child -resource resource -operation operation -state child-state.json -parent state.sjon`

To finish
`opentracing-cli -action finish -state state.json`