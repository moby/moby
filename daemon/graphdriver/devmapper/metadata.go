// +build linux,amd64

package devmapper

import (
	"encoding/xml"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

type MetadataDecoder struct {
	d      *xml.Decoder
	ranges *Ranges
}

func NewMetadataDecoder(reader io.Reader) *MetadataDecoder {
	m := &MetadataDecoder{
		d:      xml.NewDecoder(reader),
		ranges: NewRanges(),
	}

	return m
}

func (m *MetadataDecoder) parseRange(start *xml.StartElement) error {
	var begin, length uint64
	var err error
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "data_begin":
			begin, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				return err
			}
		case "length":
			length, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				return err
			}
		}
	}

	m.ranges.Add(begin, begin+length)

	m.d.Skip()
	return nil
}

func (m *MetadataDecoder) parseSingle(start *xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "data_block":
			block, err := strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				return err
			}
			m.ranges.Add(block, block+1)
		}
	}

	m.d.Skip()

	return nil
}

func (m *MetadataDecoder) parseDevice(start *xml.StartElement) error {
	for {
		tok, err := m.d.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "range_mapping":
				if err := m.parseRange(&tok); err != nil {
					return err
				}
			case "single_mapping":
				if err := m.parseSingle(&tok); err != nil {
					return err
				}
			default:
				return fmt.Errorf("Unknown tag type %s\n", tok.Name)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (m *MetadataDecoder) readStart() (*xml.StartElement, error) {
	for {
		tok, err := m.d.Token()
		if err != nil {
			return nil, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			return &tok, nil

		case xml.EndElement:
			return nil, fmt.Errorf("Unbalanced tags")
		}
	}
}

func (m *MetadataDecoder) parseMetadata() error {
	start, err := m.readStart()
	if err != nil {
		return err
	}
	if start.Name.Local != "superblock" {
		return fmt.Errorf("Unexpected tag type %s", start.Name)
	}

	for {
		tok, err := m.d.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "device":
				m.parseDevice(&tok)
			default:
				return fmt.Errorf("Unknown tag type %s\n", tok.Name)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func readMetadataRanges(file string) (*Ranges, error) {
	cmd := exec.Command("thin_dump", file)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	m := NewMetadataDecoder(stdout)

	errChan := make(chan error)

	go func() {
		err = m.parseMetadata()
		errChan <- err
	}()

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	err = <-errChan
	if err != nil {
		return nil, err
	}

	return m.ranges, nil
}
