package main

import (
	"fmt"

	"github.com/d5/tengo/v2"
	"github.com/nbd-wtf/go-nostr"
)

func eventToTengo(event *nostr.Event) tengo.Object {
	return &tengo.Map{
		Value: map[string]tengo.Object{
			"id":         &tengo.String{Value: event.ID},
			"pubkey":     &tengo.String{Value: event.PubKey},
			"sig":        &tengo.String{Value: event.Sig},
			"content":    &tengo.String{Value: event.Content},
			"kind":       &tengo.Int{Value: int64(event.Kind)},
			"created_at": &tengo.Int{Value: int64(event.CreatedAt)},
			"tags":       tagsToTengo(event.Tags),
		},
	}
}

func tagsToTengo(tags nostr.Tags) tengo.Object {
	ttags := make([]tengo.Object, len(tags))
	for t, tag := range tags {
		ttags[t] = stringSliceToTengo(tag)
	}
	return &tengo.Array{Value: ttags}
}

func stringSliceToTengo(ss []string) tengo.Object {
	tss := make([]tengo.Object, len(ss))
	for i, item := range ss {
		tss[i] = &tengo.String{Value: item}
	}
	return &tengo.Array{Value: tss}
}

func intSliceToTengo(ss []int) tengo.Object {
	tss := make([]tengo.Object, len(ss))
	for i, item := range ss {
		tss[i] = &tengo.Int{Value: int64(item)}
	}
	return &tengo.Array{Value: tss}
}

func filterToTengo(filter nostr.Filter) tengo.Object {
	f := make(map[string]tengo.Object, 8)

	if len(filter.IDs) > 0 {
		f["ids"] = stringSliceToTengo(filter.IDs)
	}
	if len(filter.Authors) > 0 {
		f["authors"] = stringSliceToTengo(filter.Authors)
	}
	if len(filter.Kinds) > 0 {
		f["kinds"] = intSliceToTengo(filter.Kinds)
	}
	for tag, values := range filter.Tags {
		f["#"+tag] = stringSliceToTengo(values)
	}
	if filter.Limit > 0 {
		f["limit"] = &tengo.Int{Value: int64(filter.Limit)}
	}
	if filter.Since != nil {
		f["since"] = &tengo.Int{Value: int64(*filter.Since)}
	}
	if filter.Until != nil {
		f["until"] = &tengo.Int{Value: int64(*filter.Until)}
	}
	if filter.Search != "" {
		f["search"] = &tengo.String{Value: filter.Search}
	}

	return &tengo.Map{Value: f}
}

func filterFromTengo(f tengo.Object) (nostr.Filter, error) {
	filter := nostr.Filter{}
	var tmap map[string]tengo.Object

	switch o := f.(type) {
	case *tengo.Map:
		tmap = o.Value
	case *tengo.ImmutableMap:
		tmap = o.Value
	default:
		return filter, fmt.Errorf("%v is not a map", f)
	}

	filter.Tags = make(nostr.TagMap)
	var err error

	for key, value := range tmap {
		switch key {
		case "ids":
			filter.IDs, err = tengoSliceToString(value)
		case "authors":
			filter.Authors, err = tengoSliceToString(value)
		case "kinds":
			filter.Kinds, err = tengoSliceToInt(value)
		case "limit":
			filter.Limit = int(value.(*tengo.Int).Value)
			if filter.Limit == 0 {
				filter.LimitZero = true
			}
		case "since":
			v := nostr.Timestamp(value.(*tengo.Int).Value)
			filter.Since = &v
		case "until":
			v := nostr.Timestamp(value.(*tengo.Int).Value)
			filter.Until = &v
		default:
			if key[0] == '#' {
				filter.Tags[key[1:]], err = tengoSliceToString(value)
			}
		}
	}

	return filter, err
}

func tengoSliceToString(v tengo.Object) ([]string, error) {
	var tss []tengo.Object

	switch v.(type) {
	case *tengo.Array:
		tss = v.(*tengo.Array).Value
	case *tengo.ImmutableArray:
		tss = v.(*tengo.Array).Value
	default:
		return nil, fmt.Errorf("%v is not an array", v)
	}

	ss := make([]string, len(tss))
	for i, o := range tss {
		ss[i] = o.(*tengo.String).Value
	}
	return ss, nil
}

func tengoSliceToInt(v tengo.Object) ([]int, error) {
	var tss []tengo.Object

	switch v.(type) {
	case *tengo.Array:
		tss = v.(*tengo.Array).Value
	case *tengo.ImmutableArray:
		tss = v.(*tengo.Array).Value
	default:
		return nil, fmt.Errorf("%v is not an array", v)
	}

	ss := make([]int, len(tss))
	for i, o := range tss {
		ss[i] = int(o.(*tengo.Int).Value)
	}
	return ss, nil
}

type EventIteratorWrapper struct {
	tengo.ObjectImpl
	ch      chan *nostr.Event
	isEnded bool
}

func (_ EventIteratorWrapper) String() string   { return "<event channel>" }
func (_ EventIteratorWrapper) TypeName() string { return "event-channel" }
func (_ EventIteratorWrapper) CanIterate() bool { return true }
func (eiw EventIteratorWrapper) Iterate() tengo.Iterator {
	return &EventIterator{w: &eiw}
}

type EventIterator struct {
	tengo.ObjectImpl
	w    *EventIteratorWrapper
	i    int
	next *nostr.Event
}

func (ei EventIterator) String() string   { return "<event channel iterator>" }
func (ei EventIterator) TypeName() string { return "event-channel-iterator" }
func (ei *EventIterator) Next() bool {
	if ei.w.isEnded {
		return false
	}
	evt, ok := <-ei.w.ch
	if !ok {
		ei.w.isEnded = true
		return false
	}
	ei.next = evt
	ei.i++
	return true
}
func (ei EventIterator) Key() tengo.Object   { return &tengo.Int{Value: int64(ei.i - 1)} }
func (ei EventIterator) Value() tengo.Object { return eventToTengo(ei.next) }
