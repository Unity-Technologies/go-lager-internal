package lager

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/logging"
)


// GcpLevelName takes a Lager level name (only the first letter matters and
// it must be upper case) and returns the corresponding value GCP uses in
// structured logging to represent the severity of such logs.  Levels are
// mapped as:
//      Not used: Alert ("700") and Emergency ("800")
//      Panic, Exit - Critical ("600")
//      Fail - Error ("500")
//      Warn - Warning ("400")
//      Note - Notice ("300")
//      Access, Info - Info ("200")
//      Trace, Debug, Obj, Guts - Debug ("100")
//      Not possible (except due to bug in Lager): Default ("0")
//
func GcpLevelName(lev string) string {
	sev := logging.Default  // This value will never get used, however.
	switch lev[0] {
	//  logging.Alert() not used
	//  logging.Emergency() not used
	case 'P': case 'E':
		sev = logging.Critical
	case 'F':
		sev = logging.Error
	case 'W':
		sev = logging.Warning
	case 'N':
		sev = logging.Notice
	case 'A': case 'I':
		sev = logging.Info
	case 'T': case 'D': case 'O': case 'G':
		sev = logging.Debug
	}
	return sev.String()
}


// GcpHtttp() returns a value for logging that GCP will recognize as details
// about an HTTP(S) request (and perhaps its response), if placed under the
// key "httpRequest".
//
// `req` must not be `nil` but `resp` and `start` can be.  `start` will not
// be modified; it is of type `*time.Time` only to make it simple to omit
// latency calculations by passing in `nil`.
//
// When using tracing, this allows GCP logging to display log lines for the
// same request (if each includes this block) together.  So this can be a
// good thing to add to a context.Context used with your logging.  For this
// to work, you must log a final message that includes all three arguments
// (as well as using GCP-compatible tracing).
//
// The following items will be logged in order, except that some can be
// omitted depending on what you pass in.
//
//      "protocol"          E.g. "HTTP/1.1"
//      "requestMethod"     E.g. "GET"
//      "requestUrl"        The original URL from the request's first line.
//      "status"            E.g. "403"
//      "requestSize"       Omitted if the request body size is not yet known.
//      "responseSize"      Omitted if `resp` is `nil` or body size not known.
//      "latency"           E.g. "0.1270s".  Omitted if `start` is `nil`.
//      "remoteIp"          E.g. "127.0.0.1"
//      "serverIp"          Not currently ever included.
//      "referer"           Omitted if there is no Referer[sic] header.
//      "userAgent"         Omitted if there is no User-Agent header.
func GcpHttp(req *http.Request, resp *http.Response, start *time.Time) RawMap {
	ua := req.Header.Get("User-Agent")
	ref := req.Header.Get("Referer")
	reqSize := req.ContentLength
	remoteIp := req.RemoteAddr
	// TODO: Add support for proxy headers.
//  if req.Header.Get("?") {
//      remoteIp = ...
//  }
	status := 0
	respSize := int64(-1)
	if nil != resp {
		status = resp.StatusCode
		respSize = resp.ContentLength
	}
	lag := ""
	if nil != start {
		lag = fmt.Sprintf("%.4fs", time.Now().Sub(*start).Seconds())
	}
	return Map(
		"protocol",                             req.Proto,
		"requestMethod",                        req.Method,
		"requestUrl",                           req.RequestURI,
		Unless(0 == status, "status"),          strconv.Itoa(status),
		Unless(reqSize < 0, "requestSize"),     strconv.FormatInt(reqSize, 10),
		Unless(respSize < 0, "responseSize"),   strconv.FormatInt(respSize, 10),
		Unless("" == lag, "latency"),           lag,
		"remoteIp",                             remoteIp,
	//  "serverIp",                             ?,
		Unless("" == ref, "referer"),           ref,
		Unless("" == ua, "userAgent"),          ua,
	)
}
