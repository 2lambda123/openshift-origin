package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"log"
)

var reLine = regexp.MustCompile("^\\s*(\\w+)\\s+(\\w+)\\s+(.+)$")

func main() {
	log.SetFlags(0)

	dir := flag.String("dir", "", "the directory that contains the binary; if empty, the same as the first argument")
	prefix := flag.String("prefix", "", "the function prefix to scan for; if not specified will use *<dirBasename>.Test*")
	help := flag.Bool("help", false, "display help")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		log.Fatalf("Must specify the name of a single directory, e.g. ./test/integration")
	}

	path := args[0]
	_ = args[1:]
	base := filepath.Base(path)

	execDir := path
	if len(*dir) > 0 {
		execDir = *dir
	}

	pkg := filepath.Join(execDir, fmt.Sprintf("%s.test", base))
	testExec, err := filepath.Abs(pkg)
	if err != nil {
		log.Fatalf("Unable to make path %q absolute: %v", execDir, err)
	}
	if _, err = os.Stat(testExec); err != nil {
		log.Fatalf("No test executable %q exits, did you run `go test -c` on the named package?", testExec)
	}

	cmd := exec.Command("go", "tool", "nm", "-sort", "name", "-n", testExec)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Can't get `go tool nm` output from %q: %v", testExec, err)
	}

	names := []string{}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		match := reLine.FindStringSubmatch(line)
		if len(match) == 0 {
			//log.Printf("mismatch: %s", line)
			continue
		}
		// ignore non code segments
		if match[2] != "t" && match[2] != "T" {
			// log.Printf("non-code line: %s",line)
			continue
		}
		name := match[3]
		// there are always two segments per function, ignore the extra one
		if strings.HasSuffix(name, ".f") {
			continue
		}

		segments := strings.SplitAfter(name, "/")
		// expect a package and name
		if len(segments) < 2 {
			//log.Printf("root_package: %s", name)
			continue
		}
		last := segments[len(segments)-1]
		_ = segments[len(segments)-2]

		// two parts
		parts := strings.Split(last, ".")
		if len(parts) != 2 {
			//log.Printf("bad_name: %s", last)
			continue
		}
		if parts[0] != base {
			//log.Printf("ignore: %s", last)
			continue
		}

		test := parts[1]
		if len(*prefix) == 0 {
			if !strings.HasPrefix(test, "Test") {
				continue
			}
		} else {
			if !strings.HasPrefix(name, *prefix) {
				continue
			}
		}
		names = append(names, test)
	}

	if len(names) == 0 {
		log.Fatalf("No tests found!")
	}
	sort.Sort(sort.StringSlice(names))
	for _, test := range names {
		fmt.Printf("%s\n", test)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Unable to scan the symbol output: %v", err)
	}
}
