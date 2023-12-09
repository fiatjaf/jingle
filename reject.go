package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fiatjaf/quickjs-go"
	"github.com/fiatjaf/quickjs-go/polyfill/pkg/console"
	"github.com/fiatjaf/quickjs-go/polyfill/pkg/fetch"
	"github.com/fiatjaf/quickjs-go/polyfill/pkg/timer"
	"github.com/nbd-wtf/go-nostr"
)

type scriptPath string

const (
	REJECT_EVENT  scriptPath = "reject-event.js"
	REJECT_FILTER scriptPath = "reject-filter.js"
)

var defaultScripts = map[scriptPath]string{
	REJECT_EVENT: `export default function (event, relay, conn) {
  if (event.kind === 0) {
    if (conn.pubkey) {
      return null
    } else {
      return 'auth-required: please auth before publishing metadata'
    }
  }

  if (event.kind !== 1) return 'we only accept kind:1 notes'
  if (event.content.length > 140)
    return 'notes must have up to 140 characters only'
  if (event.tags.length > 0) return 'notes cannot have tags'

  let metadata = relay.query({
    kinds: [0],
    authors: [event.pubkey]
  })
  if (metadata.length === 0) return 'publish your metadata here first'
}`,
	REJECT_FILTER: `export default function (filter, relay, conn) {
  if (!conn.pubkey) return "auth-required: take a selfie and send it to the CIA"

  return fetch(
    'https://www.random.org/integers/?num=1&min=1&max=9&col=1&base=10&format=plain&rnd=new'
  )
    .then(r => r.text())
    .then(res => {
      if (parseInt(res) > 4)
        return ` + "`" + `you were not lucky enough: got ${res.trim()} but needed 4 or less` + "`" + `
      return null
    })
}`,
}

func rejectEvent(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	return runAndGetResult(REJECT_EVENT,
		// first argument: the nostr event object we'll pass to the script
		func(qjs *quickjs.Context) quickjs.Value { return eventToJs(qjs, event) },
		// second argument: the relay object with goodies
		func(qjs *quickjs.Context) quickjs.Value { return makeRelayObject(ctx, qjs) },
		// third argument: the currently authenticated user
		func(qjs *quickjs.Context) quickjs.Value { return makeConnectionObject(ctx, qjs) },
	)
}

func rejectFilter(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	return runAndGetResult(REJECT_FILTER,
		// first argument: the nostr filter object we'll pass to the script
		func(qjs *quickjs.Context) quickjs.Value { return filterToJs(qjs, filter) },
		// second argument: the relay object with goodies
		func(qjs *quickjs.Context) quickjs.Value { return makeRelayObject(ctx, qjs) },
		// third argument: the currently authenticated user
		func(qjs *quickjs.Context) quickjs.Value { return makeConnectionObject(ctx, qjs) },
	)
}

func runAndGetResult(scriptPath scriptPath, makeArgs ...func(qjs *quickjs.Context) quickjs.Value) (reject bool, msg string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	rt := quickjs.NewRuntime()
	// defer rt.Close()

	qjs := rt.NewContext()
	// defer qjs.Close()

	// inject fetch and setTimeout
	fetch.InjectTo(qjs)
	timer.InjectTo(qjs)
	console.InjectTo(qjs)

	// sane defaults
	reject = true
	msg = "failed to run policy script"

	// function to get values back from js to here
	qjs.Globals().Set("____grab", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if args[0].IsString() {
			reject = true
			msg = args[0].String()
		} else if args[0].IsObject() && args[0].Has("then") {
			args[0].Call("then", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
				if args[0].IsString() {
					reject = true
					msg = args[0].String()
				} else {
					reject = false
				}
				return qjs.Null()
			}))
			rt.ExecuteAllPendingJobs()
		} else {
			reject = false
		}
		return qjs.Undefined()
	}))

	// globals
	args := qjs.Array()
	for _, makeArg := range makeArgs {
		args.Push(makeArg(qjs))
	}
	qjs.Globals().Set("args", args.ToValue())

	// register module
	code, err := os.ReadFile(filepath.Join(s.CustomDirectory, string(scriptPath)))
	if err != nil {
		log.Warn().Err(err).Str("script", string(scriptPath)).Msg("couldn't read policy file")
		return true, "couldn't read policy file"
	}
	qjs.RegisterModule(string(scriptPath), code)

	// actually run it
	val, err := qjs.Eval(`
import rejectEvent from './` + string(scriptPath) + `'
let msg = rejectEvent(...args)
____grab(msg) // this will also handle the case in which 'msg' is a promise
	`)
	defer val.Free()
	if err != nil {
		log.Warn().Err(err).Str("script", string(scriptPath)).Msg("error applying policy script")
		return true, "error applying policy script"
	}

	return reject, msg
}
