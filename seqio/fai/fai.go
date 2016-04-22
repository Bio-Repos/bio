package fai

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Record is FASTA index record
type Record struct {
	Name         string
	Length       int
	Start        int64
	BasesPerLine int
	BytesPerLine int
}

// Index is FASTA index
type Index map[string]Record

// Read faidx from .fai file
func Read(file string) (Index, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	index := make(map[string]Record)

	reader := bufio.NewReader(fh)
	var items []string
	var line, name string
	var length int
	var start int64
	var BasesPerLine, bytesPerLine int
	for {
		line, err = reader.ReadString('\n')
		if line != "" {
			line = string(dropCR([]byte(line[0 : len(line)-1])))
			items = strings.Split(line, "\t")
			if len(items) != 5 {
				return nil, fmt.Errorf("bad fai records: %s", line)
			}
			name = items[0]

			length, err = strconv.Atoi(items[1])
			if err != nil {
				return nil, err
			}

			start, err = strconv.ParseInt(items[2], 10, 64)
			if err != nil {
				return nil, err
			}

			BasesPerLine, err = strconv.Atoi(items[3])
			if err != nil {
				return nil, err
			}

			bytesPerLine, err = strconv.Atoi(items[4])
			if err != nil {
				return nil, err
			}

			index[name] = Record{
				Name:         name,
				Length:       length,
				Start:        start,
				BasesPerLine: BasesPerLine,
				BytesPerLine: bytesPerLine,
			}
		}

		if err != nil {
			break
		}
	}

	return index, nil
}

// CreateWithIDRegexp is
func CreateWithIDRegexp(file string, idRegexp string) (Index, error) {
	var err error
	IDRegexp, err = regexp.Compile(idRegexp)
	if err != nil {
		return nil, err
	}
	return Create(file)
}

// Create .fai for file
func Create(file string) (Index, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	outfh, err := os.Create(file + ".fai")
	if err != nil {
		return nil, err
	}
	defer outfh.Close()

	index := make(map[string]Record)

	reader := bufio.NewReader(fh)
	buffer := bytes.Buffer{}
	var hasSeq bool
	var lastName, thisName, sequence []byte
	var id string
	var lastStart, thisStart int64
	var lineWidths, seqWidths []int
	var lastLineWidth, lineWidth, seqWidth int
	var chances int
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			id = string(parseHeadID(lastName))

			// check lineWidths
			lastLineWidth, chances = -2, 2
			for i := len(lineWidths) - 1; i >= 0; i-- {
				if lastLineWidth == -2 {
					lastLineWidth = lineWidths[i]
					continue
				}
				if lineWidths[i] != lastLineWidth {
					chances--
					if chances == 0 {
						return nil, fmt.Errorf("different line length in sequence: %s", id)
					}
				}
				lastLineWidth = lineWidths[i]
			}
			// lineWidth = 0
			if len(lineWidths) > 0 {
				lineWidth = lineWidths[0]
			}
			// seqWidth = 0
			if len(seqWidths) > 0 {
				seqWidth = seqWidths[0]
			}

			sequence = buffer.Bytes()
			buffer.Reset()

			if _, ok := index[id]; ok {
				return index, fmt.Errorf(`ignoring duplicate sequence "%s" at byte offset %d`, id, lastStart)
			}
			outfh.WriteString(fmt.Sprintf("%s\t%d\t%d\t%d\t%d\n", id, len(sequence), lastStart, seqWidth, lineWidth))
			index[id] = Record{
				Name:         id,
				Length:       len(sequence),
				Start:        lastStart,
				BasesPerLine: seqWidth,
				BytesPerLine: lineWidth,
			}

			break
		}

		if line[0] == '>' {
			hasSeq = true
			thisName = dropCR(line[1 : len(line)-1])

			if lastName != nil {
				id = string(parseHeadID(lastName))

				// check lineWidths
				lastLineWidth, chances = -1, 2
				for i := len(lineWidths) - 1; i >= 0; i-- {
					if lastLineWidth == -1 {
						lastLineWidth = lineWidths[i]
						continue
					}
					if lineWidths[i] != lastLineWidth {
						chances--
						if chances == 0 {
							return nil, fmt.Errorf("different line length in sequence: %s", id)
						}
					}
					lastLineWidth = lineWidths[i]
				}
				// lineWidth = 0
				if len(lineWidths) > 0 {
					lineWidth = lineWidths[0]
				}
				// seqWidth = 0
				if len(seqWidths) > 0 {
					seqWidth = seqWidths[0]
				}

				sequence = buffer.Bytes()
				buffer.Reset()

				if _, ok := index[id]; ok {
					return index, fmt.Errorf(`ignoring duplicate sequence "%s" at byte offset %d`, id, lastStart)
				}
				outfh.WriteString(fmt.Sprintf("%s\t%d\t%d\t%d\t%d\n", id, len(sequence), lastStart, seqWidth, lineWidth))
				index[id] = Record{
					Name:         id,
					Length:       len(sequence),
					Start:        lastStart,
					BasesPerLine: seqWidth,
					BytesPerLine: lineWidth,
				}
			}
			lineWidths = []int{}
			seqWidths = []int{}
			thisStart += int64(len(line))
			lastStart = thisStart
			lastName = thisName
		} else if hasSeq {
			buffer.Write(dropCR(line[0 : len(line)-1]))
			thisStart += int64(len(line))

			lineWidths = append(lineWidths, len(line))
			seqWidths = append(seqWidths, len(dropCR(line[0:len(line)-1])))
		}
	}

	return index, nil
}

// ------------------------------------------------------------

// IDRegexp is regexp for parsing record id
var IDRegexp = regexp.MustCompile(`^([^\s]+)\s?`)

func parseHeadID(head []byte) []byte {
	found := IDRegexp.FindAllSubmatch(head, -1)
	if found == nil { // not match
		return head
	}
	return found[0][1]
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func cleanSeq(slice []byte) []byte {
	newSlice := []byte{}
	for _, b := range slice {
		switch b {
		case '\r', '\n':
		default:
			newSlice = append(newSlice, b)
		}
	}
	return newSlice
}