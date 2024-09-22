package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/nbd-wtf/go-nostr"
)

type scriptPath string

const (
	REJECT_EVENT  scriptPath = "reject-event.tengo"
	REJECT_FILTER scriptPath = "reject-filter.tengo"
)

var (
	rejectEventCompiled    *tengo.Compiled
	lastRejectEventModtime time.Time

	rejectFilterCompiled    *tengo.Compiled
	lastRejectFilterModtime time.Time
)

var defaultScripts = map[scriptPath]string{
	REJECT_EVENT: `export func(event, relay, conn) {
  if (event.kind == 0) {
    if (conn.get_authed_pubkey() == "") {
      return "auth-required: please auth before publishing metadata"
    } else {
      return undefined
    }
  }

  if (event.kind != 1) {
    return "we only accept kind:1 notes"
  }

  if len(event.content) > 140 {
    return "notes must have up to 140 characters only"
  }
  if len(event.tags) > 0 {
    return "notes cannot have tags"
  }

  for metadata in relay.query({ kinds: [0], authors: [event.pubkey] }) {
    // if there is a metadata event we will accept this kind:1 note
    return undefined
  }

  return "publish your metadata here first"
}`,
	REJECT_FILTER: `export func(filter, relay, conn) {
  if (!conn.pubkey) {
    return "auth-required: take a selfie and send it to the CIA"
  }

  random := get('https://www.random.org/integers/?num=1&min=1&max=9&col=1&base=10&format=plain&rnd=new')
  if int(random) > 4 {
    return "you were not lucky enough: got ${res.trim()} but needed 4 or less"
  }

  return undefined
}`,
}

func rejectEvent(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	fpath := filepath.Join(s.CustomDirectory, string(REJECT_EVENT))
	fstat, err := os.Stat(fpath)
	if err != nil {
		return true, "couldn't find script file"
	}

	var this *tengo.Compiled
	if fstat.ModTime().After(lastRejectEventModtime) {
		lastRejectEventModtime = fstat.ModTime()
		script := tengo.NewScript([]byte(`
reject := import("userscript")
res := reject(event, relay, conn)
`))

		modules := tengo.NewModuleMap()
		for name, mod := range stdlib.SourceModules {
			modules.AddSourceModule(name, []byte(mod))
		}
		for name, mod := range stdlib.BuiltinModules {
			modules.AddBuiltinModule(name, mod)
		}
		source, _ := os.ReadFile(fpath)
		modules.AddSourceModule("userscript", source)

		script.SetImports(modules)
		script.Add("event", nil)
		script.Add("relay", nil)
		script.Add("conn", nil)

		rejectEventCompiled, err = script.Compile()
		if err != nil {
			return true, "script is invalid: " + err.Error()
		}

		this = rejectEventCompiled.Clone()
	} else {
		this = rejectEventCompiled.Clone()
	}

	this.Set("event", eventToTengo(event))
	this.Set("relay", makeRelayObject(ctx))
	this.Set("conn", makeConnectionObject(ctx))
	if err := this.RunContext(ctx); err != nil {
		return true, "script failed to run: " + err.Error()
	}

	res := this.Get("res")
	if res.String() == "" {
		return false, ""
	} else {
		return true, res.String()
	}
}

func rejectFilter(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	fpath := filepath.Join(s.CustomDirectory, string(REJECT_FILTER))
	fstat, err := os.Stat(fpath)
	if err != nil {
		return true, "couldn't find script file"
	}

	var this *tengo.Compiled
	if fstat.ModTime().After(lastRejectEventModtime) {
		lastRejectEventModtime = fstat.ModTime()
		script := tengo.NewScript([]byte(`
reject := import("userscript")
res := reject(filter, relay, conn)
`))

		modules := tengo.NewModuleMap()
		for name, mod := range stdlib.SourceModules {
			modules.AddSourceModule(name, []byte(mod))
		}
		for name, mod := range stdlib.BuiltinModules {
			modules.AddBuiltinModule(name, mod)
		}
		source, _ := os.ReadFile(fpath)
		modules.AddSourceModule("userscript", source)
		modules.AddBuiltinModule("http", tengoHttp)

		script.SetImports(modules)
		script.Add("filter", nil)
		script.Add("relay", nil)
		script.Add("conn", nil)

		rejectEventCompiled, err = script.Compile()
		if err != nil {
			return true, "script is invalid: " + err.Error()
		}

		this = rejectEventCompiled.Clone()
	} else {
		this = rejectEventCompiled.Clone()
	}

	this.Set("filter", filterToTengo(filter))
	this.Set("relay", makeRelayObject(ctx))
	this.Set("conn", makeConnectionObject(ctx))
	if err := this.RunContext(ctx); err != nil {
		return true, "script failed to run: " + err.Error()
	}

	res := this.Get("res")
	if res.String() == "" {
		return false, ""
	} else {
		return true, res.String()
	}
}
