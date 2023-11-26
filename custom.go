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
	REJECT_EVENT: `export default function (event) {
  if (event.kind !== 1) return 'we only accept kind:1 notes'
  if (event.content.length > 140)
    return 'notes must have up to 140 characters only'
  if (event.tags.length > 0) return 'notes cannot have tags'
}`,
	REJECT_FILTER: `export default function (filter) {
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
	return runAndGetResult(REJECT_EVENT, func(qjs *quickjs.Context) quickjs.Value {
		// first argument: the nostr event object we'll pass to the script
		jsEvent := qjs.Object()
		jsEvent.Set("id", qjs.String(event.ID))
		jsEvent.Set("pubkey", qjs.String(event.PubKey))
		jsEvent.Set("sig", qjs.String(event.Sig))
		jsEvent.Set("content", qjs.String(event.Content))
		jsEvent.Set("kind", qjs.Int32(int32(event.Kind)))
		jsEvent.Set("created_at", qjs.Int64(int64(event.CreatedAt)))
		jsTags := qjs.Array()
		for _, tag := range event.Tags {
			jsTags.Push(qjsStringArray(qjs, tag))
		}
		jsEvent.Set("tags", jsTags.ToValue())
		return jsEvent
	})
}

func rejectFilter(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	return runAndGetResult(REJECT_FILTER, func(qjs *quickjs.Context) quickjs.Value {
		// first argument: the nostr filter object we'll pass to the script
		jsFilter := qjs.Object()

		if len(filter.IDs) > 0 {
			jsFilter.Set("ids", qjsStringArray(qjs, filter.IDs))
		}
		if len(filter.Authors) > 0 {
			jsFilter.Set("authors", qjsStringArray(qjs, filter.Authors))
		}
		if len(filter.Kinds) > 0 {
			jsFilter.Set("kinds", qjsIntArray(qjs, filter.Kinds))
		}
		for tag, values := range filter.Tags {
			jsFilter.Set("#"+tag, qjsStringArray(qjs, values))
		}
		if filter.Limit > 0 {
			jsFilter.Set("limit", qjs.Int32(int32(filter.Limit)))
		}
		if filter.Since != nil {
			jsFilter.Set("since", qjs.Int64(int64(*filter.Since)))
		}
		if filter.Until != nil {
			jsFilter.Set("until", qjs.Int64(int64(*filter.Until)))
		}
		if filter.Search != "" {
			jsFilter.Set("search", qjs.String(filter.Search))
		}

		return jsFilter
	})
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
	code, err := os.ReadFile(filepath.Join(s.ScriptsDirectory, string(scriptPath)))
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

func qjsStringArray(qjs *quickjs.Context, src []string) quickjs.Value {
	arr := qjs.Array()
	for _, item := range src {
		arr.Push(qjs.String(item))
	}
	return arr.ToValue()
}

func qjsIntArray(qjs *quickjs.Context, src []int) quickjs.Value {
	arr := qjs.Array()
	for _, item := range src {
		arr.Push(qjs.Int32(int32(item)))
	}
	return arr.ToValue()
}
