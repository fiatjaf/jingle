package main

import (
	"context"
	"os"

	"github.com/nbd-wtf/go-nostr"
	"github.com/quickjs-go/quickjs-go"
)

func rejectEvent(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
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
	qjs.Globals().Set("grab", qjs.Function(func(qjs *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if args[0].IsString() {
			reject = true
			msg = args[0].String()
		}
		return qjs.Undefined()
	}))

	// build the nostr event object we'll pass to the script
	jsEvent := qjs.Object()
	jsEvent.Set("id", qjs.String(event.ID))
	jsEvent.Set("pubkey", qjs.String(event.PubKey))
	jsEvent.Set("sig", qjs.String(event.Sig))
	jsEvent.Set("content", qjs.String(event.Content))
	jsEvent.Set("kind", qjs.Int32(int32(event.Kind)))
	jsEvent.Set("created_at", qjs.Int64(int64(event.CreatedAt)))
	jsTags := qjs.Array()
	for i, tag := range event.Tags {
		jsTag := qjs.Array()
		for j, item := range tag {
			jsTag.SetByUint32(uint32(j), qjs.String(item))
		}
		jsTags.SetByUint32(uint32(i), jsTag)
	}
	jsEvent.Set("tags", jsTags)
	qjs.Globals().Set("event", jsEvent)

	val, err := qjs.EvalFile(string(code), quickjs.EVAL_MODULE, "reject-event.js")
	defer val.Free()
	if err != nil {
		return true, "error reading policy script"
	}

	val, err = qjs.Eval(`
import rejectEvent from './reject-event.js'
let msg = rejectEvent(event)
grab(msg)
	`, quickjs.EVAL_MODULE)
	defer val.Free()
	if err != nil {
		return true, "error applying policy script"
	}

	return reject, msg
}
