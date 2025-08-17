package dummyclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/containerd/log"
	events "github.com/docker/go-events"

	"github.com/moby/moby/v2/daemon/libnetwork/diagnostic"
	"github.com/moby/moby/v2/daemon/libnetwork/networkdb"
)

type Mux interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func RegisterDiagnosticHandlers(mux Mux, nDB *networkdb.NetworkDB) {
	mux.HandleFunc("/watchtable", watchTable(nDB))
	mux.HandleFunc("/watchedtableentries", watchTableEntries)
}

const (
	missingParameter = "missing parameter"
)

type tableHandler struct {
	cancelWatch func()
	entries     map[string]string
}

var clientWatchTable = map[string]tableHandler{}

func watchTable(nDB *networkdb.NetworkDB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm() //nolint:errcheck
		diagnostic.DebugHTTPForm(r)
		if len(r.Form["tname"]) < 1 {
			rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name", r.URL.Path))
			diagnostic.HTTPReply(w, rsp, &diagnostic.JSONOutput{}) //nolint:errcheck
			return
		}

		tableName := r.Form["tname"][0]
		if _, ok := clientWatchTable[tableName]; ok {
			fmt.Fprintf(w, "OK\n")
			return
		}

		ch, cancel := nDB.Watch(tableName, "")
		clientWatchTable[tableName] = tableHandler{cancelWatch: cancel, entries: make(map[string]string)}
		go handleTableEvents(tableName, ch)

		fmt.Fprintf(w, "OK\n")
	}
}

func watchTableEntries(w http.ResponseWriter, r *http.Request) {
	r.ParseForm() //nolint:errcheck
	diagnostic.DebugHTTPForm(r)
	if len(r.Form["tname"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name", r.URL.Path))
		diagnostic.HTTPReply(w, rsp, &diagnostic.JSONOutput{}) //nolint:errcheck
		return
	}

	tableName := r.Form["tname"][0]
	table, ok := clientWatchTable[tableName]
	if !ok {
		fmt.Fprintf(w, "Table %s not watched\n", tableName)
		return
	}

	fmt.Fprintf(w, "total elements: %d\n", len(table.entries))
	i := 0
	for k, v := range table.entries {
		fmt.Fprintf(w, "%d) k:`%s` -> v:`%s`\n", i, k, v)
		i++
	}
}

func handleTableEvents(tableName string, ch *events.Channel) {
	log.G(context.TODO()).Infof("Started watching table:%s", tableName)
	for {
		select {
		case <-ch.Done():
			log.G(context.TODO()).Infof("End watching %s", tableName)
			return

		case evt := <-ch.C:
			log.G(context.TODO()).Infof("Received new event on:%s", tableName)
			event, ok := evt.(networkdb.WatchEvent)
			if !ok {
				log.G(context.TODO()).Fatalf("Unexpected table event = %#v", event)
			}
			if event.Value != nil {
				// log.G(ctx).Infof("Add %s %s", tableName, event.Key)
				clientWatchTable[tableName].entries[event.Key] = string(event.Value)
			} else {
				// log.G(ctx).Infof("Del %s %s", tableName, event.Key)
				delete(clientWatchTable[tableName].entries, event.Key)
			}
		}
	}
}
