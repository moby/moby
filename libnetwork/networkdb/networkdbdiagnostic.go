package networkdb

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/internal/caller"
	"github.com/sirupsen/logrus"
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
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("join cluster")

	if len(r.Form["members"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?members=ip1,ip2,...", r.URL.Path))
		log.Error("join cluster failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		err := nDB.Join(strings.Split(r.Form["members"][0], ","))
		if err != nil {
			rsp := diagnostic.FailCommand(fmt.Errorf("%s error in the DB join %s", r.URL.Path, err))
			log.WithError(err).Error("join cluster failed")
			diagnostic.HTTPReply(w, rsp, json)
			return
		}

		log.Info("join cluster done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbPeers(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("network peers")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=test", r.URL.Path))
		log.Error("network peers failed, wrong input")
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
		log.WithField("response", fmt.Sprintf("%+v", rsp)).Info("network peers done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbClusterPeers(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("cluster peers")

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		peers := nDB.ClusterPeers()
		rsp := &diagnostic.TableObj{Length: len(peers)}
		for i, peerInfo := range peers {
			rsp.Elements = append(rsp.Elements, &diagnostic.PeerEntryObj{Index: i, Name: peerInfo.Name, IP: peerInfo.IP})
		}
		log.WithField("response", fmt.Sprintf("%+v", rsp)).Info("cluster peers done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbCreateEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("create entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 ||
		len(r.Form["value"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k&value=v", r.URL.Path))
		log.Error("create entry failed, wrong input")
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
			log.WithError(err).Error("create entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.CreateEntry(tname, nid, key, decodedValue); err != nil {
			rsp := diagnostic.FailCommand(err)
			diagnostic.HTTPReply(w, rsp, json)
			log.WithError(err).Error("create entry failed")
			return
		}
		log.Info("create entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbUpdateEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("update entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 ||
		len(r.Form["value"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k&value=v", r.URL.Path))
		log.Error("update entry failed, wrong input")
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
			log.WithError(err).Error("update entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
	}

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.UpdateEntry(tname, nid, key, decodedValue); err != nil {
			log.WithError(err).Error("update entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		log.Info("update entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbDeleteEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("delete entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k", r.URL.Path))
		log.Error("delete entry failed, wrong input")
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
			log.WithError(err).Error("delete entry failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		log.Info("delete entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbGetEntry(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("get entry")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 ||
		len(r.Form["key"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id&key=k", r.URL.Path))
		log.Error("get entry failed, wrong input")
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
			log.WithError(err).Error("get entry failed")
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
		log.WithField("response", fmt.Sprintf("%+v", rsp)).Info("get entry done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbJoinNetwork(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("join network")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=network_id", r.URL.Path))
		log.Error("join network failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.JoinNetwork(nid); err != nil {
			log.WithError(err).Error("join network failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		log.Info("join network done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbLeaveNetwork(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("leave network")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=network_id", r.URL.Path))
		log.Error("leave network failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		if err := nDB.LeaveNetwork(nid); err != nil {
			log.WithError(err).Error("leave network failed")
			diagnostic.HTTPReply(w, diagnostic.FailCommand(err), json)
			return
		}
		log.Info("leave network done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(nil), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbGetTable(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	unsafe, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("get table")

	if len(r.Form["tname"]) < 1 ||
		len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?tname=table_name&nid=network_id", r.URL.Path))
		log.Error("get table failed, wrong input")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}

	tname := r.Form["tname"][0]
	nid := r.Form["nid"][0]

	nDB, ok := ctx.(*NetworkDB)
	if ok {
		table := nDB.GetTableByNetwork(tname, nid)
		rsp := &diagnostic.TableObj{Length: len(table)}
		var i = 0
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
		log.WithField("response", fmt.Sprintf("%+v", rsp)).Info("get table done")
		diagnostic.HTTPReply(w, diagnostic.CommandSucceed(rsp), json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}

func dbNetworkStats(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	diagnostic.DebugHTTPForm(r)
	_, json := diagnostic.ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnostic", "remoteIP": r.RemoteAddr, "method": caller.Name(0), "url": r.URL.String()})
	log.Info("network stats")

	if len(r.Form["nid"]) < 1 {
		rsp := diagnostic.WrongCommand(missingParameter, fmt.Sprintf("%s?nid=test", r.URL.Path))
		log.Error("network stats failed, wrong input")
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
		log.WithField("response", fmt.Sprintf("%+v", rsp)).Info("network stats done")
		diagnostic.HTTPReply(w, rsp, json)
		return
	}
	diagnostic.HTTPReply(w, diagnostic.FailCommand(fmt.Errorf(dbNotAvailable)), json)
}
