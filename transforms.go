package main

import (
	"github.com/nbd-wtf/go-nostr"
)

func eventToTengo(event *nostr.Event) map[string]any {
	return map[string]any{
		"id":         event.ID,
		"pubkey":     event.PubKey,
		"sig":        event.Sig,
		"content":    event.Content,
		"kind":       int32(event.Kind),
		"created_at": int64(event.CreatedAt),
		"tags":       event.Tags,
	}
}

func filterToTengo(filter nostr.Filter) map[string]any {
	f := make(map[string]any, 8)

	if len(filter.IDs) > 0 {
		f["ids"] = filter.IDs
	}
	if len(filter.Authors) > 0 {
		f["authors"] = filter.Authors
	}
	if len(filter.Kinds) > 0 {
		f["kinds"] = filter.Kinds
	}
	for tag, values := range filter.Tags {
		f["#"+tag] = values
	}
	if filter.Limit > 0 {
		f["limit"] = int32(filter.Limit)
	}
	if filter.Since != nil {
		f["since"] = int64(*filter.Since)
	}
	if filter.Until != nil {
		f["until"] = int64(*filter.Until)
	}
	if filter.Search != "" {
		f["search"] = filter.Search
	}

	return f
}

func filterFromTengo(f map[string]any) nostr.Filter {
	filter := nostr.Filter{}
	filter.Tags = make(nostr.TagMap)

	for key, value := range f {
		switch key {
		case "ids":
			filter.IDs = value.([]string)
		case "authors":
			filter.Authors = value.([]string)
		case "kinds":
			filter.Kinds = value.([]int)
		case "limit":
			filter.Limit = value.(int)
			if filter.Limit == 0 {
				filter.LimitZero = true
			}
		case "since":
			v := nostr.Timestamp(value.(int64))
			filter.Since = &v
		case "until":
			v := nostr.Timestamp(value.(int64))
			filter.Until = &v
		default:
			if key[0] == '#' {
				filter.Tags[key[1:]] = value.([]string)
			}
		}
	}
	return filter
}
