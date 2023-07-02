package dummyclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/networkdb"
	events "github.com/docker/go-events"
)

// DummyClientPaths2Func exported paths for the client
var DummyClientPaths2Func = map[string]diagnostic.HTTPHandlerFunc{
	"/watchtable":          watchTable,
	"/watchedtableentries": watchTableEntries,
}

const (
	missingParameter = "missing parameter"
)

type tableHandler struct {
	cancelWatch func()
	entries     map[string]string
}

var clientWatchTable = map[string]tableHandler{}

func watchTable(ctx interface{}, w http.ResponseWriter, r *http.Request) {
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

	nDB, ok := ctx.(*networkdb.NetworkDB)
	if ok {
		ch, cancel := nDB.Watch(tableName, "")
		clientWatchTable[tableName] = tableHandler{cancelWatch: cancel, entries: make(map[string]string)}
		go handleTableEvents(tableName, ch)

		fmt.Fprintf(w, "OK\n")
	}
}

func watchTableEntries(ctx interface{}, w http.ResponseWriter, r *http.Request) {
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
	var (
		// nid   string
		eid   string
		value []byte
		isAdd bool
	)

	log.G(context.TODO()).Infof("Started watching table:%s", tableName)
	for {
		select {
		case <-ch.Done():
			log.G(context.TODO()).Infof("End watching %s", tableName)
			return

		case evt := <-ch.C:
			log.G(context.TODO()).Infof("Recevied new event on:%s", tableName)
			switch event := evt.(type) {
			case networkdb.CreateEvent:
				// nid = event.NetworkID
				eid = event.Key
				value = event.Value
				isAdd = true
			case networkdb.DeleteEvent:
				// nid = event.NetworkID
				eid = event.Key
				value = event.Value
				isAdd = false
			default:
				log.G(context.TODO()).Fatalf("Unexpected table event = %#v", event)
			}
			if isAdd {
				// log.G(ctx).Infof("Add %s %s", tableName, eid)
				clientWatchTable[tableName].entries[eid] = string(value)
			} else {
				// log.G(ctx).Infof("Del %s %s", tableName, eid)
				delete(clientWatchTable[tableName].entries, eid)
			}
		}
	}
}
