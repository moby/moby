package networkdb

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/internal/caller"
)

const (
	missingParameter = "missing parameter"
	dbNotAvailable   = "database not available"
)

// NetDbPaths2Func TODO
var NetDbPaths2Func = map[string]diagnostic.HTTPHandlerFunc{
	"/join":         dbJoin,
	"/networkpeers": dbPeers,
	"/clusterpeers": dbClusterPeers,
	"/joinnetwork":  dbJoinNetwork,
	"/leavenetwork": dbLeaveNetwork,
	"/createentry":  dbCreateEntry,
	"/updateentry":  dbUpdateEntry,
	"/deleteentry":  dbDeleteEntry,
	"/getentry":     dbGetEntry,
	"/gettable":     dbGetTable,
	"/networkstats": dbNetworkStats,
}

func dbJoin(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("join cluster")

	if len(r.Form["members"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?members=ip1,ip2,...", r.URL.Path))
		logger.Error("join cluster failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		err := nDB.Join(strings.Split(r.Form["members"][0], ","))
		if err != nil {
			rsp := diagnostic.FailCommand(fmt.Errorf("%s error in the DB join %s", r.URL.Path, err))
			logger.WithError(err).Error("join cluster failed")
			diagnostic.HTTPReply(w, rsp, json)
			return
		}

		logger.Info("join cluster done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbPeers(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("network peers")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=test", r.URL.Path))
		logger.Error("network peers failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		peers := nDB.Peers(r.Form["nid"][0])
		rsp := &diagnostic.TableObj{Length: len(peers)}
		for i, peerInfo := range peers {
			if peerInfo.IP == "unknown" {
				rsp.Elements = append(rsp.Elements, &diagnostic.PeerEntryObj{Index: i, Name: "orphan-" + peerInfo.Name, IP: peerInfo.IP})
			} else {
				rsp.Elements = append(rsp.Elements, &diagnostic.PeerEntryObj{Index: i, Name: peerInfo.Name, IP: peerInfo.IP})
			}
		}
		logger.WithField("response", fmt.Sprintf("%+v", rsp)).Info("network peers done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbClusterPeers(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("cluster peers")

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		peers := nDB.ClusterPeers()
		rsp := &diagnostic.TableObj{Length: len(peers)}
		for i, peerInfo := range peers {
			rsp.Elements = append(rsp.Elements, &diagnostic.PeerEntryObj{Index: i, Name: peerInfo.Name, IP: peerInfo.IP})
		}
		logger.WithField("response", fmt.Sprintf("%+v", rsp)).Info("cluster peers done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbCreateEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("create entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 ||
		len(r.Form["value"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k&value=v", r.URL.Path))
		logger.Error("create entry failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]
	key := r.Form["key"][0]
	value := r.Form["value"][0]
	decodedValue := []byte(value)
	if !unsafe {
		var err error
		decodedValue, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			logger.WithError(err).Error("create entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.CreateEntry(tname, nid, key, decodedValue); err != nil {
			rsp := diagnostic.FailCommand(err)
			diagnostic.HTTPReply(w, rsp, json)
			logger.WithError(err).Error("create entry failed")
			return
		}
		logger.Info("create entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbUpdateEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("update entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 ||
		len(r.Form["value"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k&value=v", r.URL.Path))
		logger.Error("update entry failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]
	key := r.Form["key"][0]
	value := r.Form["value"][0]
	decodedValue := []byte(value)
	if !unsafe {
		var err error
		decodedValue, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			logger.WithError(err).Error("update entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.UpdateEntry(tname, nid, key, decodedValue); err != nil {
			logger.WithError(err).Error("update entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		logger.Info("update entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbDeleteEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("delete entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k", r.URL.Path))
		logger.Error("delete entry failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]
	key := r.Form["key"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		err := nDB.DeleteEntry(tname, nid, key)
		if err != nil {
			logger.WithError(err).Error("delete entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		logger.Info("delete entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbGetEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("get entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k", r.URL.Path))
		logger.Error("get entry failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]
	key := r.Form["key"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		value, err := nDB.GetEntry(tname, nid, key)
		if err != nil {
			logger.WithError(err).Error("get entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}

		var encodedValue string
		if unsafe {
			encodedValue = string(value)
		} else {
			encodedValue = base64.StdEncoding.EncodeToString(value)
		}

		rsp := &diagnostic.TableEntryObj{Key: key, Value: encodedValue}
		logger.WithField("response", fmt.Sprintf("%+v", rsp)).Info("get entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbJoinNetwork(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("join network")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=network_id", r.URL.Path))
		logger.Error("join network failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.JoinNetwork(nid); err != nil {
			logger.WithError(err).Error("join network failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		logger.Info("join network done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbLeaveNetwork(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("leave network")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=network_id", r.URL.Path))
		logger.Error("leave network failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.LeaveNetwork(nid); err != nil {
			logger.WithError(err).Error("leave network failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		logger.Info("leave network done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbGetTable(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("get table")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id", r.URL.Path))
		logger.Error("get table failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		table := nDB.GetTableByNetwork(tname, nid)
		rsp := &diagnostic.TableObj{Length: len(table)}
		i := 0
		for k, v := range table {
			var encodedValue string
			if unsafe {
				encodedValue = string(v.Value)
			} else {
				encodedValue = base64.StdEncoding.EncodeToString(v.Value)
			}
			rsp.Elements = append(rsp.Elements,
				&diagnostic.TableEntryObj{
					Index: i,
					Key:   k,
					Value: encodedValue,
					Owner: v.owner,
				})
			i++
		}
		logger.WithField("response", fmt.Sprintf("%+v", rsp)).Info("get table done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbNetworkStats(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	})
	logger.Info("network stats")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=test", r.URL.Path))
		logger.Error("network stats failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		nDB.RLock()
		networks := nDB.networks[nDB.config.NodeID]
		network, ok := networks[r.Form["nid"][0]]

		entries := -1
		qLen := -1
		if ok {
			entries = int(network.entriesNumber.Load())
			qLen = network.tableBroadcasts.NumQueued()
		}
		nDB.RUnlock()

		rsp := diagnostic.CommandSucceed(&diagnostic.NetworkStatsResult{Entries: entries, QueueLen: qLen})
		logger.WithField("response", fmt.Sprintf("%+v", rsp)).Info("network stats done")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}
