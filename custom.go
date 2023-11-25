package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/nbd-wtf/go-nostr"
	"github.com/quickjs-go/quickjs-go"
)

func rejectEvent(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	return runAndGetResult("scripts/reject-event.js", func(qjs *quickjs.Context) quickjs.Value {
		// first argument: the nostr event object we'll pass to the script
		jsEvent := qjs.Object()
		jsEvent.Set("id", qjs.String(event.ID))
		jsEvent.Set("pubkey", qjs.String(event.PubKey))
		jsEvent.Set("sig", qjs.String(event.Sig))
		jsEvent.Set("content", qjs.String(event.Content))
		jsEvent.Set("kind", qjs.Int32(int32(event.Kind)))
		jsEvent.Set("created_at", qjs.Int64(int64(event.CreatedAt)))
		jsTags := qjs.Array()
		for i, tag := range event.Tags {
			jsTags.SetByUint32(uint32(i), qjsStringArray(qjs, tag))
		}
		jsEvent.Set("tags", jsTags)
		return jsEvent
	})
}

func rejectFilter(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	return runAndGetResult("scripts/reject-filter.js", func(qjs *quickjs.Context) quickjs.Value {
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

func runAndGetResult(scriptPath string, makeArgs ...func(qjs *quickjs.Context) quickjs.Value) (reject bool, msg string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	rt := quickjs.NewRuntime()
	defer rt.Free()

	qjs := rt.NewContext()
	defer qjs.Free()

	// read code from user file
	code, err := os.ReadFile("scripts/reject-event.js")
	if err != nil {
		return true, "missing policy"
	}

	// function to get values back from js to here
	qjs.Globals().Set("____grab", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		fmt.Println("grabbing")
		if args[0].IsString() {
			reject = true
			msg = args[0].String()
		}
		return qjs.Undefined()
	}))

	// standard library
	qjs.Globals().Set("fetch", qjs.Function(fetchFunc))

	// globals
	args := qjs.Array()
	for i, makeArg := range makeArgs {
		args.SetByUint32(uint32(i), makeArg(qjs))
	}
	qjs.Globals().Set("args", args)

	val, err := qjs.EvalFile(string(code), quickjs.EVAL_MODULE, "reject-event.js")
	defer val.Free()
	if err != nil {
		log.Warn().Err(err).Str("script", scriptPath).Msg("error reading policy script")
		return true, "error reading policy script"
	}

	val, err = qjs.Eval(`
import rejectEvent from './`+scriptPath+`'
let msg = rejectEvent(...args)
console.log("msg", msg)
if (msg.then) {
  console.log("will grab")
  msg.then(____grab)
} else {
  ____grab(msg)
}
	`, quickjs.EVAL_MODULE)

	fmt.Println("done")
	defer val.Free()
	if err != nil {
		log.Warn().Err(err).Str("script", scriptPath).Msg("error applying policy script")
		return true, "error applying policy script"
	}

	return reject, msg
}
