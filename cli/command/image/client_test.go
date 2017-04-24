package image

import (
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	imageTagFunc     func(string, string) error
	imageSaveFunc    func(images []string) (io.ReadCloser, error)
	imageRemoveFunc  func(image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	imagePushFunc    func(ref string, options types.ImagePushOptions) (io.ReadCloser, error)
	infoFunc         func() (types.Info, error)
	imagePullFunc    func(ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	imagesPruneFunc  func(pruneFilter filters.Args) (types.ImagesPruneReport, error)
	imageLoadFunc    func(input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	imageListFunc    func(options types.ImageListOptions) ([]types.ImageSummary, error)
	imageInspectFunc func(image string) (types.ImageInspect, []byte, error)
	imageImportFunc  func(source types.ImageImportSource, ref string, options types.ImageImportOptions) (io.ReadCloser, error)
	imageHistoryFunc func(image string) ([]image.HistoryResponseItem, error)
}

func (cli *fakeClient) ImageTag(_ context.Context, image, ref string) error {
	if cli.imageTagFunc != nil {
		return cli.imageTagFunc(image, ref)
	}
	return nil
}

func (cli *fakeClient) ImageSave(_ context.Context, images []string) (io.ReadCloser, error) {
	if cli.imageSaveFunc != nil {
		return cli.imageSaveFunc(images)
	}
	return ioutil.NopCloser(strings.NewReader("")), nil
}

func (cli *fakeClient) ImageRemove(_ context.Context, image string,
	options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	if cli.imageRemoveFunc != nil {
		return cli.imageRemoveFunc(image, options)
	}
	return []types.ImageDeleteResponseItem{}, nil
}

func (cli *fakeClient) ImagePush(_ context.Context, ref string, options types.ImagePushOptions) (io.ReadCloser, error) {
	if cli.imagePushFunc != nil {
		return cli.imagePushFunc(ref, options)
	}
	return ioutil.NopCloser(strings.NewReader("")), nil
}

func (cli *fakeClient) Info(_ context.Context) (types.Info, error) {
	if cli.infoFunc != nil {
		return cli.infoFunc()
	}
	return types.Info{}, nil
}

func (cli *fakeClient) ImagePull(_ context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	if cli.imagePullFunc != nil {
		cli.imagePullFunc(ref, options)
	}
	return ioutil.NopCloser(strings.NewReader("")), nil
}

func (cli *fakeClient) ImagesPrune(_ context.Context, pruneFilter filters.Args) (types.ImagesPruneReport, error) {
	if cli.imagesPruneFunc != nil {
		return cli.imagesPruneFunc(pruneFilter)
	}
	return types.ImagesPruneReport{}, nil
}

func (cli *fakeClient) ImageLoad(_ context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
	if cli.imageLoadFunc != nil {
		return cli.imageLoadFunc(input, quiet)
	}
	return types.ImageLoadResponse{}, nil
}

func (cli *fakeClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	if cli.imageListFunc != nil {
		return cli.imageListFunc(options)
	}
	return []types.ImageSummary{{}}, nil
}

func (cli *fakeClient) ImageInspectWithRaw(_ context.Context, image string) (types.ImageInspect, []byte, error) {
	if cli.imageInspectFunc != nil {
		return cli.imageInspectFunc(image)
	}
	return types.ImageInspect{}, nil, nil
}

func (cli *fakeClient) ImageImport(_ context.Context, source types.ImageImportSource, ref string,
	options types.ImageImportOptions) (io.ReadCloser, error) {
	if cli.imageImportFunc != nil {
		return cli.imageImportFunc(source, ref, options)
	}
	return ioutil.NopCloser(strings.NewReader("")), nil
}

func (cli *fakeClient) ImageHistory(_ context.Context, img string) ([]image.HistoryResponseItem, error) {
	if cli.imageHistoryFunc != nil {
		return cli.imageHistoryFunc(img)
	}
	return []image.HistoryResponseItem{{ID: img, Created: time.Now().Unix()}}, nil
}
