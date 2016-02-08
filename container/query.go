package container

import (
	"fmt"
	"strings"

	"strconv"

	"github.com/docker/docker/query"
)

var (
	ContainerQueryFields = map[string][]query.Operator{
		"running":    {query.IS},
		"paused":     {query.IS},
		"restarting": {query.IS},

		"label.*": {query.IS, query.EQ, query.LIKE},

		"id":         {query.EQ, query.LIKE},
		"name":       {query.EQ, query.LIKE},
		"image":      {query.EQ, query.LIKE},
		"cmd":        {query.EQ, query.LIKE},
		"entrypoint": {query.EQ, query.LIKE},

		"exit":    {query.EQ, query.GT},
		"created": {query.EQ, query.GT},
		"exited":  {query.EQ, query.GT},
	}
)

var _ query.Queryable = &Container{}

func (c *Container) Is(field string, operator query.Operator, value string) bool {
	switch {
	case field == "running":
		return c.Running
	case field == "paused":
		return c.Paused
	case field == "restarting":
		return c.Restarting
	case strings.HasPrefix(field, "label."):
		label := strings.TrimPrefix(field, "label.")
		labelValue, found := c.Config.Labels[label]
		if operator == query.IS {
			return found
		}
		return strCompare(labelValue, operator, value)
	case field == "id":
		return strCompare(c.ID, operator, value)
	case field == "name":
		return strCompare(strings.TrimPrefix(c.Name, "/"), operator, value)
	case field == "image":
		//FIXME: how to retrieve image name ?
		return strCompare(c.ImageID.String(), operator, value)
	case field == "exit":
		code := c.ExitCode
		return code != -1 && intCompare(code, operator, value)
	case field == "cmd":
		return sliceCompare(c.Config.Cmd.Slice(), operator, value)
	case field == "entrypoint":
		return sliceCompare(c.Config.Entrypoint.Slice(), operator, value)
	default:
		panic(fmt.Sprintf("Invalid field %s", field))
	}
}

func intCompare(value int, op query.Operator, pattern string) bool {
	ipattern, err := strconv.Atoi(pattern)
	if err != nil {
		panic(fmt.Sprintf("'%s' is not a numeric", op))
	}
	switch op {
	case query.EQ:
		return value == ipattern
	case query.GT:
		return value > ipattern
	default:
		panic(fmt.Sprintf("Unsupported operator %s", op))
	}
}

func strCompare(value string, op query.Operator, pattern string) bool {
	switch op {
	case query.EQ:
		return value == pattern
	case query.LIKE:
		return like(value, pattern)
	default:
		panic(fmt.Sprintf("Unsupported operator %s", op))
	}
}

func sliceCompare(values []string, op query.Operator, pattern string) bool {
	for _, value := range values {
		switch op {
		case query.EQ:
			if value == pattern {
				return true
			}
		case query.LIKE:
			if like(value, pattern) {
				return true
			}
		default:
			panic(fmt.Sprintf("Unsupported operator %s", op))
		}
	}
	return false
}

func like(value, pattern string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
}
