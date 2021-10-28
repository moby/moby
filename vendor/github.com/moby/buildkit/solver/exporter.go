package solver

import (
	"context"

	digest "github.com/opencontainers/go-digest"
)

type exporter struct {
	k       *CacheKey
	records []*CacheRecord
	record  *CacheRecord

	edge     *edge // for secondaryExporters
	override *bool
}

func addBacklinks(t CacheExporterTarget, rec CacheExporterRecord, cm *cacheManager, id string, bkm map[string]CacheExporterRecord) (CacheExporterRecord, error) {
	if rec == nil {
		var ok bool
		rec, ok = bkm[id]
		if ok && rec != nil {
			return rec, nil
		}
		_ = ok
	}
	bkm[id] = nil
	if err := cm.backend.WalkBacklinks(id, func(id string, link CacheInfoLink) error {
		if rec == nil {
			rec = t.Add(link.Digest)
		}
		r, ok := bkm[id]
		if !ok {
			var err error
			r, err = addBacklinks(t, nil, cm, id, bkm)
			if err != nil {
				return err
			}
		}
		if r != nil {
			rec.LinkFrom(r, int(link.Input), link.Selector.String())
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if rec == nil {
		rec = t.Add(digest.Digest(id))
	}
	bkm[id] = rec
	return rec, nil
}

type contextT string

var backlinkKey = contextT("solver/exporter/backlinks")
var resKey = contextT("solver/exporter/res")

func (e *exporter) ExportTo(ctx context.Context, t CacheExporterTarget, opt CacheExportOpt) ([]CacheExporterRecord, error) {
	var bkm map[string]CacheExporterRecord

	if bk := ctx.Value(backlinkKey); bk == nil {
		bkm = map[string]CacheExporterRecord{}
		ctx = context.WithValue(ctx, backlinkKey, bkm)
	} else {
		bkm = bk.(map[string]CacheExporterRecord)
	}

	var res map[*exporter][]CacheExporterRecord
	if r := ctx.Value(resKey); r == nil {
		res = map[*exporter][]CacheExporterRecord{}
		ctx = context.WithValue(ctx, resKey, res)
	} else {
		res = r.(map[*exporter][]CacheExporterRecord)
	}

	if t.Visited(e) {
		return res[e], nil
	}
	t.Visit(e)

	deps := e.k.Deps()

	type expr struct {
		r        CacheExporterRecord
		selector digest.Digest
	}

	rec := t.Add(rootKey(e.k.Digest(), e.k.Output()))
	allRec := []CacheExporterRecord{rec}

	addRecord := true

	if e.override != nil {
		addRecord = *e.override
	}

	if e.record == nil && len(e.k.Deps()) > 0 {
		e.record = getBestResult(e.records)
	}

	var remote *Remote
	if v := e.record; v != nil && len(e.k.Deps()) > 0 && addRecord {
		cm := v.cacheManager
		key := cm.getID(v.key)
		res, err := cm.backend.Load(key, v.ID)
		if err != nil {
			return nil, err
		}

		remote, err = cm.results.LoadRemote(ctx, res, opt.Session)
		if err != nil {
			return nil, err
		}

		if remote == nil && opt.Mode != CacheExportModeRemoteOnly {
			res, err := cm.results.Load(ctx, res)
			if err != nil {
				return nil, err
			}
			remote, err = opt.Convert(ctx, res)
			if err != nil {
				return nil, err
			}
			res.Release(context.TODO())
		}

		if remote != nil {
			for _, rec := range allRec {
				rec.AddResult(v.CreatedAt, remote)
			}
		}
	}

	if remote != nil && opt.Mode == CacheExportModeMin {
		opt.Mode = CacheExportModeRemoteOnly
	}

	srcs := make([][]expr, len(deps))

	for i, deps := range deps {
		for _, dep := range deps {
			recs, err := dep.CacheKey.Exporter.ExportTo(ctx, t, opt)
			if err != nil {
				return nil, nil
			}
			for _, r := range recs {
				srcs[i] = append(srcs[i], expr{r: r, selector: dep.Selector})
			}
		}
	}

	if e.edge != nil {
		for _, de := range e.edge.secondaryExporters {
			recs, err := de.cacheKey.CacheKey.Exporter.ExportTo(ctx, t, opt)
			if err != nil {
				return nil, nil
			}
			for _, r := range recs {
				srcs[de.index] = append(srcs[de.index], expr{r: r, selector: de.cacheKey.Selector})
			}
		}
	}

	for i, srcs := range srcs {
		for _, src := range srcs {
			rec.LinkFrom(src.r, i, src.selector.String())
		}
	}

	for cm, id := range e.k.ids {
		if _, err := addBacklinks(t, rec, cm, id, bkm); err != nil {
			return nil, err
		}
	}

	if v := e.record; v != nil && len(deps) == 0 {
		cm := v.cacheManager
		key := cm.getID(v.key)
		if err := cm.backend.WalkIDsByResult(v.ID, func(id string) error {
			if id == key {
				return nil
			}
			allRec = append(allRec, t.Add(digest.Digest(id)))
			return nil
		}); err != nil {
			return nil, err
		}
	}

	res[e] = allRec

	return allRec, nil
}

func getBestResult(records []*CacheRecord) *CacheRecord {
	var rec *CacheRecord
	for _, r := range records {
		if rec == nil || rec.CreatedAt.Before(r.CreatedAt) || (rec.CreatedAt.Equal(r.CreatedAt) && rec.Priority < r.Priority) {
			rec = r
		}
	}
	return rec
}

type mergedExporter struct {
	exporters []CacheExporter
}

func (e *mergedExporter) ExportTo(ctx context.Context, t CacheExporterTarget, opt CacheExportOpt) (er []CacheExporterRecord, err error) {
	for _, e := range e.exporters {
		r, err := e.ExportTo(ctx, t, opt)
		if err != nil {
			return nil, err
		}
		er = append(er, r...)
	}
	return
}
