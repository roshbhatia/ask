package schema

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// offered lists one spelling per type; typeOf parses more (str, text, integer).
var offered = []struct {
	kind string
	says string
}{
	{"string", "any text"},
	{"int", "a whole number"},
	{"number", "a number, whole or not"},
	{"bool", "true or false"},
	{"object", "a nested object"},
	{"any", "anything at all"},
	{"[]string", "a list of text"},
	{"[]int", "a list of whole numbers"},
	{"[]number", "a list of numbers"},
	{"[]bool", "a list of true or false"},
	{"[]object", "a list of objects"},
}

var starters = []struct {
	spec string
	says string
}{
	{"ok:bool, reason:string", "did it work, and why not"},
	{"files:[]string", "a list of paths"},
	{"level:error|warn|info, message:string", "a bar makes an enum"},
	{"name:string, count:int?", "a trailing ? makes a field optional"},
}

// Complete offers the rest of a --schema spec. A field name is free text, so
// only the type is offered. The second answer asks the shell to complete a path.
func Complete(typed string) (offer []string, paths bool) {
	if strings.HasPrefix(typed, "@") {
		return nil, true
	}
	if strings.TrimSpace(typed) == "" {
		for _, one := range starters {
			offer = append(offer, one.spec+"\t"+one.says)
		}
		return offer, false
	}

	head, last := lastField(typed)
	name, partial, typing := strings.Cut(last, ":")
	if !typing {
		return nil, false
	}

	for _, one := range offered {
		if strings.HasPrefix(one.kind, partial) {
			offer = append(offer, head+name+":"+one.kind+"\t"+one.says)
		}
	}
	return offer, false
}

// CompletePaths offers filesystem entries while preserving the @ schema-file prefix.
func CompletePaths(typed string) []string {
	path := strings.TrimPrefix(typed, "@")
	displayDirectory, prefix := filepath.Split(path)
	readDirectory := displayDirectory
	if strings.HasPrefix(readDirectory, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil {
			readDirectory = filepath.Join(home, strings.TrimPrefix(readDirectory, "~"+string(filepath.Separator)))
		}
	}
	if readDirectory == "" {
		readDirectory = "."
	}
	entries, err := os.ReadDir(readDirectory)
	if err != nil {
		return nil
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		candidate := "@" + displayDirectory + name
		if entry.IsDir() {
			candidate += string(filepath.Separator)
		}
		values = append(values, candidate)
	}
	sort.Strings(values)
	return values
}

// lastField cuts the spec at the last comma, so earlier fields carry through.
func lastField(typed string) (head string, last string) {
	at := strings.LastIndex(typed, ",")
	if at < 0 {
		return "", typed
	}
	return typed[:at+1], typed[at+1:]
}
