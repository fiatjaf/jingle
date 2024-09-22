package main

import (
	"context"
	"sync"

	"github.com/fiatjaf/khatru"
	"github.com/puzpuzpuz/xsync/v2"
)

var sessionStorage = xsync.NewTypedMapOf[*khatru.WebSocket, store](pointerHasher)

type store struct {
	data  map[string]any
	mutex sync.Mutex
}

var globalStore = store{data: make(map[string]any)}

func onDisconnect(ctx context.Context) {
	sessionStorage.Delete(khatru.GetConnection(ctx))
}

func makeRelayObject(ctx context.Context) map[string]any {
	return map[string]any{
		"query": func(f map[string]any) []map[string]any {
			filter := filterFromTengo(f)
			events, err := wrapper.QuerySync(ctx, filter)
			if err != nil {
				panic(err)
			}
			results := make([]map[string]any, len(events))
			for i, event := range events {
				results[i] = eventToTengo(event)
			}
			return results
		},
		"store": map[string]any{
			"get": func(key string) any {
				globalStore.mutex.Lock()
				defer globalStore.mutex.Unlock()
				return globalStore.data[key]
			},
			"set": func(key string, value any) {
				globalStore.mutex.Lock()
				globalStore.data[key] = value
				globalStore.mutex.Unlock()
			},
			"del": func(key string) {
				globalStore.mutex.Lock()
				defer globalStore.mutex.Unlock()
				delete(globalStore.data, key)
			},
		},
	}
}

func makeConnectionObject(ctx context.Context) map[string]any {
	return map[string]any{
		"ip":     khatru.GetIP(ctx),
		"pubkey": khatru.GetAuthed(ctx),
		"store": map[string]any{
			"get": func(key string) any {
				store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
					return store{data: make(map[string]any)}
				})
				store.mutex.Lock()
				defer store.mutex.Unlock()
				return store.data[key]
			},
			"set": func(key string, value any) {
				store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
					return store{data: make(map[string]any)}
				})
				store.mutex.Lock()
				store.data[key] = value
				store.mutex.Unlock()
			},
			"del": func(key string) {
				store, _ := sessionStorage.LoadOrCompute(khatru.GetConnection(ctx), func() store {
					return store{data: make(map[string]any)}
				})
				store.mutex.Lock()
				defer store.mutex.Unlock()
				delete(store.data, key)
			},
		},
	}
}
