package gitobject

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // used for git object hashes
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type GitObject struct {
	Type       string
	Headers    map[string][]string
	Message    string
	Signature  string
	SignedData string
	Raw        []byte
}

type Actor struct {
	Name  string
	Email string
	When  *time.Time
}

type Commit struct {
	Tree      string
	Parents   []string
	Author    Actor
	Committer Actor
	Message   string
}

type Tag struct {
	Object  string
	Type    string
	Tag     string
	Tagger  Actor
	Message string
}

func Parse(raw []byte) (*GitObject, error) {
	obj := &GitObject{Headers: make(map[string][]string), Raw: raw}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 {
		return nil, errors.Errorf("invalid empty git object")
	}

	isTag := bytes.HasPrefix(raw, []byte("object "))
	if isTag {
		obj.Type = "tag"
	} else {
		obj.Type = "commit"
	}

	var headersDone bool
	var messageDone bool
	var sigLines []string
	var messageLines []string
	var signedDataLines []string
	inSig := false

	for _, l := range lines {
		if !headersDone {
			if l == "" {
				headersDone = true
				signedDataLines = append(signedDataLines, l)
				continue
			}
			if !isTag && strings.HasPrefix(l, "gpgsig ") {
				inSig = true
				sigLines = append(sigLines, strings.TrimPrefix(l, "gpgsig "))
				continue
			}
			if !isTag && strings.HasPrefix(l, "gpgsig-sha256 ") {
				inSig = true
				sigLines = append(sigLines, strings.TrimPrefix(l, "gpgsig-sha256 "))
				continue
			}
			if inSig {
				if v, ok := strings.CutPrefix(l, " "); ok {
					sigLines = append(sigLines, v)
					continue
				} else {
					inSig = false
				}
			}
			signedDataLines = append(signedDataLines, l)
			parts := strings.SplitN(l, " ", 2)
			if len(parts) == 2 {
				obj.Headers[parts[0]] = append(obj.Headers[parts[0]], parts[1])
			}
			continue
		}
		if isTag && (l == "-----BEGIN PGP SIGNATURE-----" || l == "-----BEGIN SSH SIGNATURE-----") {
			messageDone = true
		}
		if messageDone {
			sigLines = append(sigLines, l)
		} else {
			messageLines = append(messageLines, l)
			signedDataLines = append(signedDataLines, l)
		}
	}

	obj.Message = strings.Join(messageLines, "\n")
	obj.Message = strings.TrimSuffix(obj.Message, "\n") // body ends with newline but no extra newline between message and signature
	if len(sigLines) > 0 {
		obj.Signature = strings.Join(sigLines, "\n")
	}

	obj.SignedData = strings.Join(signedDataLines, "\n")
	if isTag {
		obj.SignedData += "\n"
	}

	// basic validation
	requiredHeaders := []string{}
	switch obj.Type {
	case "commit":
		requiredHeaders = append(requiredHeaders, "tree", "author", "committer")
	case "tag":
		requiredHeaders = append(requiredHeaders, "object", "type", "tag", "tagger")
	}

	for _, header := range requiredHeaders {
		if _, ok := obj.Headers[header]; !ok {
			return nil, errors.Errorf("invalid %s object: missing %s header", obj.Type, header)
		}
	}

	return obj, nil
}

func (obj *GitObject) Checksum(hashFunc func() hash.Hash) ([]byte, error) {
	h := hashFunc()
	header := fmt.Sprintf("commit %d\u0000", len(obj.Raw))
	if obj.Type == "tag" {
		header = fmt.Sprintf("tag %d\u0000", len(obj.Raw))
	}
	data := append([]byte(header), obj.Raw...)
	h.Write(data)
	return h.Sum(nil), nil
}

func (obj *GitObject) VerifyChecksum(sha string) error {
	var hf func() hash.Hash
	switch len(sha) {
	case 40:
		hf = sha1.New
	case 64:
		hf = sha256.New
	default:
		return errors.Errorf("unsupported sha length %d", len(sha))
	}
	sum, err := obj.Checksum(hf)
	if err != nil {
		return err
	}
	if hexValue := hex.EncodeToString(sum); sha != hexValue {
		return errors.Errorf("checksum mismatch: expected %s, got %s", sha, hexValue)
	}
	return nil
}

func (obj *GitObject) ToCommit() (*Commit, error) {
	if obj.Type != "commit" {
		return nil, errors.Errorf("not a commit object")
	}
	c := &Commit{}
	if trees, ok := obj.Headers["tree"]; ok && len(trees) > 0 {
		c.Tree = trees[0]
	}
	if parents, ok := obj.Headers["parent"]; ok && len(parents) > 0 {
		c.Parents = parents
	}
	if authors, ok := obj.Headers["author"]; ok && len(authors) > 0 {
		c.Author = parseActor(authors[0])
	}
	if committers, ok := obj.Headers["committer"]; ok && len(committers) > 0 {
		c.Committer = parseActor(committers[0])
	}
	c.Message = obj.Message
	return c, nil
}

func (obj *GitObject) ToTag() (*Tag, error) {
	if obj.Type != "tag" {
		return nil, errors.Errorf("not a tag object")
	}
	t := &Tag{}
	if objects, ok := obj.Headers["object"]; ok && len(objects) > 0 {
		t.Object = objects[0]
	}
	if types, ok := obj.Headers["type"]; ok && len(types) > 0 {
		t.Type = types[0]
	}
	if tags, ok := obj.Headers["tag"]; ok && len(tags) > 0 {
		t.Tag = tags[0]
	}
	if taggers, ok := obj.Headers["tagger"]; ok && len(taggers) > 0 {
		t.Tagger = parseActor(taggers[0])
	}
	t.Message = obj.Message
	return t, nil
}

func parseActor(s string) Actor {
	s = strings.TrimSpace(s)
	var a Actor

	// find last angle brackets, because name can contain '<'
	start := strings.LastIndex(s, "<")
	end := strings.LastIndex(s, ">")
	if start == -1 || end == -1 || end < start {
		// malformed, treat as plain name
		a.Name = s
		return a
	}

	a.Name = strings.TrimSpace(s[:start])
	a.Email = strings.TrimSpace(s[start+1 : end])

	rest := strings.TrimSpace(s[end+1:])
	if rest == "" {
		return a
	}

	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return a
	}

	unix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return a
	}

	tz := parts[1]
	if len(tz) != 5 || (tz[0] != '+' && tz[0] != '-') {
		return a
	}

	sign := 1
	if tz[0] == '-' {
		sign = -1
	}
	hh, _ := strconv.Atoi(tz[1:3])
	mm, _ := strconv.Atoi(tz[3:5])
	offset := sign * (hh*3600 + mm*60)

	t := time.Unix(unix, 0).In(time.FixedZone("", offset))
	a.When = &t
	return a
}
