package formatter

import (
	"strings"
	"time"

	"github.com/docker/docker/api/types"
)

type imageSorter struct {
	imageCtxs []*imageContext
	images    []types.ImageSummary
	by        string
}

func (sorter imageSorter) Len() int { return len(sorter.imageCtxs) }

func (sorter imageSorter) Swap(i, j int) {
	sorter.imageCtxs[i], sorter.imageCtxs[j] = sorter.imageCtxs[j], sorter.imageCtxs[i]
}

func (sorter imageSorter) Less(i, j int) bool {
	ctxi := sorter.imageCtxs[i]
	ctxj := sorter.imageCtxs[j]
	sorter.by = strings.Trim(sorter.by, " ")
	if len(sorter.by) == 0 {
		sorter.by = "created:des"
	}
	switch sorter.by {
	case "size":
		fallthrough
	case "size:asc":
		return ctxi.i.Size <= ctxj.i.Size
	case "size:des":
		return ctxi.i.Size > ctxj.i.Size
	case "repo":
		fallthrough
	case "repo:asc":
		repoTagi := ctxi.repo + ctxi.tag
		repoTagj := ctxj.repo + ctxj.tag
		if strings.Compare(repoTagi, repoTagj) <= 0 {
			return true
		}
		return false
	case "repo:des":
		repoTagi := ctxi.repo + ctxi.tag
		repoTagj := ctxj.repo + ctxj.tag
		if strings.Compare(repoTagi, repoTagj) > 0 {
			return true
		}
		return false
	case "created":
		fallthrough
	case "created:asc":
		timei := time.Unix(int64(ctxi.i.Created), 0)
		timej := time.Unix(int64(ctxj.i.Created), 0)
		if timei.Before(timej) || timei.Equal(timej) {
			return true
		}
		return false
	case "created:des":
		timei := time.Unix(int64(ctxi.i.Created), 0)
		timej := time.Unix(int64(ctxj.i.Created), 0)
		if timei.After(timej) {
			return true
		}
		return false
	}

	return true
}
