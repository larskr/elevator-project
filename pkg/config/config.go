// This package reads a very minimal config file format into a Go map.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func LoadFile(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	c := make(map[string]string)

	r := bufio.NewReader(f)
	var line, prefix string
	for err != io.EOF {
		line, err = r.ReadString('\n')
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			prefix = ""
			continue
		}

		if line[0] == '[' {
			i := 1
			for ; line[i] != ']'; i++ {
			}
			prefix = line[1:i]
		} else {
			i := 0
			for ; line[i] != ' ' && i < len(line); i++ {
			}
			field := line[:i]
			for ; line[i] != '=' && i < len(line); i++ {
			}
			val := strings.TrimSpace(line[i+1:])
			if prefix != "" {
				str := fmt.Sprintf("%v.%v", prefix, field)
				c[str] = val
			} else {
				c[field] = val
			}
		}
	}

	return c, nil
}
