package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"strings"
)

type Link struct {
	From  string
	To    string
	Addr  string
	Alias string
	Port  string
}

type LinkRepository struct {
	links map[string]Link
}

func (l *Link) ToEnv() string {
	return fmt.Sprintf("%s_ADDR=%s", strings.ToUpper(l.Alias), l.Addr)
}

func NewLinkRepository(root string) (*LinkRepository, error) {
	r := &LinkRepository{make(map[string]Link)}
	return r, nil
}

// Return all links for a container
func (l *LinkRepository) Get(id string) []Link {
	id = strings.Trim(strings.ToLower(id), "")
	out := []Link{}
	for _, link := range l.links {
		if link.To == id || link.From == id {
			out = append(out, link)
		}
	}
	return out
}

// Returns the link for a current alias
func (l *LinkRepository) GetByAlias(alias string) (Link, error) {
	link, exists := l.links[alias]
	if !exists {
		return link, fmt.Errorf("Link does not exist for alias: %s", alias)
	}
	return link, nil
}

// Create a new link with a unique alias
func (l *LinkRepository) RegisterLink(link Link) error {
	if _, exists := l.links[link.Alias]; exists {
		return fmt.Errorf("A link for %s already exists", link.Alias)
	}
	utils.Debugf("Registering link: %v", link)
	l.links[link.Alias] = link
	return nil
}
