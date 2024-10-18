package hub

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SearchResult struct {
	Total   int                `json:"total"`
	Results []SearchResultItem `json:"results"`
}

type SearchResultItemCategory struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type SearchResultItem struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Slug              string                     `json:"slug"`
	Type              string                     `json:"type"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
	ShortDesc         string                     `json:"short_description"`
	Source            string                     `json:"source"`
	StarCount         int                        `json:"star_count"`
	Categories        []SearchResultItemCategory `json:"categories,omitempty"`
	ExtensionReviewed bool                       `json:"extension_reviewed,omitempty"`
	Archived          bool                       `json:"archived,omitempty"`
}

type Image struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Creator             int       `json:"creator"`
	LastUpdated         time.Time `json:"last_updated"`
	LastUpdater         int       `json:"last_updater"`
	LastUpdaterUsername string    `json:"last_updater_username"`
	Repository          int       `json:"repository"`
	FullSize            int       `json:"full_size"`
	V2                  bool      `json:"v2"`
	TagStatus           string    `json:"tag_status"`
	TagLastPulled       time.Time `json:"tag_last_pulled"`
	TagLastPushed       time.Time `json:"tag_last_pushed"`
	MediaType           string    `json:"media_type"`
	ContentType         string    `json:"content_type"`
	Digest              string    `json:"digest"`
}

type ImageTags struct {
	Count    int     `json:"count"`
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Results  []Image `json:"results"`
}

type ImageOptions struct {
	PrivilegeFunc func(context.Context) (string, error)

	// Name is the tag of the image to search for.
	Name string
	// Ordering is the order of the search "last_updated"
	Ordering string
	// Page is the page number of the search.
	Page int
	// PageSize is the number of results to return on the selected page.
	PageSize int
}

func (i *ImageOptions) ToQuery(q url.Values) url.Values {
	q.Set("name", i.Name)
	q.Set("ordering", i.Ordering)
	q.Set("page", strconv.Itoa(i.Page))
	q.Set("page_size", strconv.Itoa(i.PageSize))
	return q
}

type SearchOrder string

const (
	SearchOrderAsc  SearchOrder = "asc"
	SearchOrderDesc SearchOrder = "desc"
)

type SearchSource string

const (
	SearchSourceStore     SearchSource = "store"
	SearchSourceCommunity SearchSource = "community"
)

type SearchType string

const (
	SearchTypeImage     SearchType = "image"
	SearchTypePlugin    SearchType = "plugin"
	SearchTypeExtension SearchType = "extension"
)

type SearchSort string

const (
	SearchSortName      SearchSort = "name"
	SearchSortUpdatedAt SearchSort = "updated_at"
)

type SearchOptions struct {
	PrivilegeFunc func(context.Context) (string, error)

	// From is the index of the first result to return
	From int
	// Size is the number of results to return
	Size int
	// Order is the order of the search "asc" or "desc"
	Order SearchOrder
	// Sort is the field to sort the search by "name" or "updated_at"
	Sort SearchSort
	// Source can be "store" or "community"
	Source SearchSource
	// Type can be "image", "plugin" or "extension."
	Type SearchType
	// Categories is a list of categories to filter the search.
	Categories []string
	// OperatingSystems is a list of operating systems to filter the search. Supported values include "linux" and "windows".
	OperatingSystems []string
	// Architectures is a list of architectures to filter the search. Supported values include "amd64", "arm", "arm64", "386", "ppc64le", "s390x", "riscv64", "mips64le".
	Architectures []string
	// ExtensionReviewed filters the search to only include extensions that have been reviewed by Docker Hub.
	ExtensionReviewed bool
	// OpenSource filters the search to only include open source images.
	OpenSource bool
	// Certified filters the search to only include certified images.
	Certified bool
	// Official filters the search to only include Docker Official Images (DOI).
	Official bool
}

func (s *SearchOptions) ToQuery(q url.Values) url.Values {
	q.Set("from", strconv.Itoa(s.From))
	q.Set("size", strconv.Itoa(s.Size))
	q.Set("order", string(s.Order))
	q.Set("sort", string(s.Sort))
	q.Set("source", string(s.Source))
	q.Set("type", string(s.Type))
	q.Set("categories", strings.Join(s.Categories, ","))
	q.Set("operating_systems", strings.Join(s.OperatingSystems, ","))
	q.Set("architectures", strings.Join(s.Architectures, ","))
	q.Set("extension_reviewed", strconv.FormatBool(s.ExtensionReviewed))
	q.Set("open_source", strconv.FormatBool(s.OpenSource))
	q.Set("official", strconv.FormatBool(s.Official))
	q.Set("certified", strconv.FormatBool(s.Certified))
	return q
}

func (s *SearchOptions) FromQuery(q url.Values) error {
	var err error

	if from := q.Get("from"); from != "" {
		s.From, err = strconv.Atoi(from)
		if err != nil {
			return err
		}
	}
	if size := q.Get("size"); size != "" {
		s.Size, err = strconv.Atoi(size)
		if err != nil {
			return err
		}
	}
	if order := q.Get("order"); order != "" {
		s.Order = SearchOrder(order)
	}
	if sort := q.Get("sort"); sort != "" {
		s.Sort = SearchSort(sort)
	}
	if source := q.Get("source"); source != "" {
		s.Source = SearchSource(source)
	}
	if t := q.Get("type"); t != "" {
		s.Type = SearchType(t)
	}
	if categories := q.Get("categories"); categories != "" {
		s.Categories = strings.Split(categories, ",")
	}
	if operatingSystems := q.Get("operating_systems"); operatingSystems != "" {
		s.OperatingSystems = strings.Split(operatingSystems, ",")
	}
	if architectures := q.Get("architectures"); architectures != "" {
		s.Architectures = strings.Split(architectures, ",")
	}
	if extensionReviewed := q.Get("extension_reviewed"); extensionReviewed != "" {
		s.ExtensionReviewed, err = strconv.ParseBool(extensionReviewed)
		if err != nil {
			return err
		}
	}
	if openSource := q.Get("open_source"); openSource != "" {
		s.OpenSource, err = strconv.ParseBool(openSource)
		if err != nil {
			return err
		}
	}
	if certified := q.Get("certified"); certified != "" {
		s.Certified, err = strconv.ParseBool(certified)
		if err != nil {
			return err
		}
	}
	if official := q.Get("official"); official != "" {
		s.Official, err = strconv.ParseBool(official)
		if err != nil {
			return err
		}
	}
	return nil
}
