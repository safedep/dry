// Command gen writes list.json, the SPDX License List data that the
// spdxlicense package embeds. It reads licenses.json and exceptions.json of
// github.com/spdx/license-list-data at one pinned commit, so a run gives the
// same file until a person changes the commit.
//
// To move to a newer list, set commit to the license-list-data commit of the
// new build, run "go run ./policy/spdxlicense/internal/gen" from the root of
// the module, and change the version, the date and the commit in NOTICE.
// Read the diff of list.json. The command reads the network, and CI runs
// "go generate" on each change, so no generate directive runs it.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

// commit is the license-list-data commit "License list build 297da51",
// released 2026-01-30.
const commit = "7e1f010ac88e4b8b58bc889e9a9bf42cf6624273"

const baseURL = "https://raw.githubusercontent.com/spdx/license-list-data/" + commit + "/json/"

// output is relative to the root of the module, where the command runs.
const output = "policy/spdxlicense/list.json"

type licenseSource struct {
	Version     string `json:"licenseListVersion"`
	ReleaseDate string `json:"releaseDate"`
	Licenses    []struct {
		ID         string `json:"licenseId"`
		Deprecated bool   `json:"isDeprecatedLicenseId"`
		OSI        bool   `json:"isOsiApproved"`
		FSF        bool   `json:"isFsfLibre"`
	} `json:"licenses"`
}

type exceptionSource struct {
	Version    string `json:"licenseListVersion"`
	Exceptions []struct {
		ID         string `json:"licenseExceptionId"`
		Deprecated bool   `json:"isDeprecatedLicenseId"`
	} `json:"exceptions"`
}

type license struct {
	ID         string `json:"id"`
	Deprecated bool   `json:"deprecated,omitempty"`
	OSI        bool   `json:"osi,omitempty"`
	FSF        bool   `json:"fsf,omitempty"`
}

type list struct {
	Version     string    `json:"version"`
	ReleaseDate string    `json:"release_date"`
	Licenses    []license `json:"licenses"`
	Exceptions  []string  `json:"exceptions"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	var licenses licenseSource
	if err := fetch("licenses.json", &licenses); err != nil {
		return err
	}

	var exceptions exceptionSource
	if err := fetch("exceptions.json", &exceptions); err != nil {
		return err
	}

	if licenses.Version != exceptions.Version {
		return fmt.Errorf("licenses.json is list %s and exceptions.json is list %s", licenses.Version, exceptions.Version)
	}

	out := list{Version: licenses.Version, ReleaseDate: licenses.ReleaseDate}
	for _, l := range licenses.Licenses {
		out.Licenses = append(out.Licenses, license{ID: l.ID, Deprecated: l.Deprecated, OSI: l.OSI, FSF: l.FSF})
	}
	sort.Slice(out.Licenses, func(i, j int) bool { return out.Licenses[i].ID < out.Licenses[j].ID })

	// A deprecated exception has no successor that the parser could map it
	// to, so the parser does not accept one.
	for _, e := range exceptions.Exceptions {
		if !e.Deprecated {
			out.Exceptions = append(out.Exceptions, e.ID)
		}
	}
	sort.Strings(out.Exceptions)

	b, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, append(b, '\n'), 0o644)
}

func fetch(name string, v any) error {
	client := &http.Client{Timeout: time.Minute}
	resp, err := client.Get(baseURL + name)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", name, err)
	}
	// The body is read in full before the close, so a close error loses no
	// data.
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", name, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}
