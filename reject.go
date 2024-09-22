package main

import (
	"context"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/nbd-wtf/go-nostr"
)

type scriptPath string

const (
	REJECT_EVENT  scriptPath = "reject-event.tengo"
	REJECT_FILTER scriptPath = "reject-filter.tengo"
)

var defaultScripts = map[scriptPath]string{
	REJECT_EVENT: `export func(event, relay, conn) {
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
	REJECT_FILTER: `export func(filter, relay, conn) {
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
	script := tengo.NewScript([]byte(`
reject := import("` + REJECT_EVENT + `")
res := reject(event, relay, conn)
`))
	script.SetImportDir(s.CustomDirectory)
	script.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
	script.Add("event", eventToTengo(event))
	script.Add("relay", makeRelayObject(ctx))
	script.Add("conn", makeConnectionObject(ctx))

	compiled, err := script.RunContext(ctx)
	if err != nil {
		return true, "script failed to run: " + msg
	}

	res := compiled.Get("result")
	if res.String() == "" {
		return false, ""
	} else {
		return true, res.String()
	}
}

func rejectFilter(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	script := tengo.NewScript([]byte(`
reject := import("` + REJECT_FILTER + `")
res := reject(filter, relay, conn)
`))
	script.SetImportDir(s.CustomDirectory)
	script.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
	script.Add("filter", filterToTengo(filter))
	script.Add("relay", makeRelayObject(ctx))
	script.Add("conn", makeConnectionObject(ctx))

	compiled, err := script.RunContext(ctx)
	if err != nil {
		return true, "script failed to run: " + msg
	}

	res := compiled.Get("result")
	if res.String() == "" {
		return false, ""
	} else {
		return true, res.String()
	}
}
