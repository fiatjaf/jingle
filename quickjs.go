package main

import (
	"github.com/fiatjaf/quickjs-go"
	"github.com/nbd-wtf/go-nostr"
)

func eventToJs(qjs *quickjs.Context, event *nostr.Event) quickjs.Value {
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
}

func filterToJs(qjs *quickjs.Context, filter nostr.Filter) quickjs.Value {
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
}

func filterFromJs(qjs *quickjs.Context, jsFilter quickjs.Value) nostr.Filter {
	filter := nostr.Filter{}
	filter.Tags = make(nostr.TagMap)

	keys, _ := jsFilter.PropertyNames()
	for _, key := range keys {
		switch key {
		case "ids":
			filter.IDs = qjsReadStringArray(jsFilter.Get("ids"))
		case "authors":
			filter.Authors = qjsReadStringArray(jsFilter.Get("authors"))
		case "kinds":
			filter.Kinds = qjsReadIntArray(jsFilter.Get("kinds"))
		case "limit":
			filter.Limit = int(jsFilter.Get(key).Int64())
		case "since":
			v := nostr.Timestamp(jsFilter.Get(key).Int64())
			filter.Until = &v
		case "until":
			v := nostr.Timestamp(jsFilter.Get(key).Int64())
			filter.Until = &v
		default:
			if key[0] == '#' {
				filter.Tags[key[1:]] = qjsReadStringArray(jsFilter.Get(key))
			}
		}
	}
	return filter
}

func qjsStringArray(qjs *quickjs.Context, src []string) quickjs.Value {
	arr := qjs.Array()
	for _, item := range src {
		arr.Push(qjs.String(item))
	}
	return arr.ToValue()
}

func qjsReadStringArray(arr quickjs.Value) []string {
	strs := make([]string, arr.Len())
	for i := 0; i < len(strs); i++ {
		strs[i] = arr.GetIdx(int64(i)).String()
	}
	return strs
}

func qjsReadIntArray(arr quickjs.Value) []int {
	ints := make([]int, arr.Len())
	for i := 0; i < len(ints); i++ {
		ints[i] = int(arr.GetIdx(int64(i)).Int32())
	}
	return ints
}

func qjsIntArray(qjs *quickjs.Context, src []int) quickjs.Value {
	arr := qjs.Array()
	for _, item := range src {
		arr.Push(qjs.Int32(int32(item)))
	}
	return arr.ToValue()
}
