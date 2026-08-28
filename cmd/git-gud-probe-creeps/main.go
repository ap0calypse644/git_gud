package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
)

type classProbe struct {
	ClassName    string   `json:"class_name"`
	Created      int      `json:"created"`
	Updated      int      `json:"updated"`
	SampleFields []string `json:"sample_fields"`
}

type output struct {
	ReplayPath string       `json:"replay_path"`
	Classes    []classProbe `json:"classes"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-probe-creeps:", err)
		os.Exit(1)
	}
}

func run() error {
	replayPath := flag.String("replay", "", "path to a decompressed Source 2 .dem replay")
	flag.Parse()
	if *replayPath == "" {
		return fmt.Errorf("-replay is required")
	}

	f, err := os.Open(*replayPath)
	if err != nil {
		return fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		return fmt.Errorf("create manta parser: %w", err)
	}

	classes := map[string]*classProbe{}
	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		className := e.GetClassName()
		if !possibleLaneCreepClass(className) {
			return nil
		}

		probe := classes[className]
		if probe == nil {
			probe = &classProbe{ClassName: className, SampleFields: []string{}}
			classes[className] = probe
		}
		if op.Flag(manta.EntityOpCreated) {
			probe.Created++
		}
		if op.Flag(manta.EntityOpUpdated) {
			probe.Updated++
		}

		if len(probe.SampleFields) == 0 {
			fields := e.Map()
			probe.SampleFields = make([]string, 0, len(fields))
			for key := range fields {
				probe.SampleFields = append(probe.SampleFields, key)
			}
			sort.Strings(probe.SampleFields)
		}
		return nil
	})

	if err := p.Start(); err != nil {
		return fmt.Errorf("parse replay: %w", err)
	}

	out := output{ReplayPath: *replayPath, Classes: make([]classProbe, 0, len(classes))}
	for _, probe := range classes {
		out.Classes = append(out.Classes, *probe)
	}
	sort.Slice(out.Classes, func(i, j int) bool { return out.Classes[i].ClassName < out.Classes[j].ClassName })

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	return nil
}

func possibleLaneCreepClass(className string) bool {
	name := strings.ToLower(className)
	return strings.Contains(name, "creep") || strings.Contains(name, "lane")
}
