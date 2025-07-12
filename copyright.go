// Copyright (c) 2016-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
)

const (
	single = "single"
	multi  = "multi"
	hash   = "hash"
)

func main() {
	xos.AppName = "Copyright"
	xos.AppVersion = "1.2.0"
	xos.CopyrightStartYear = "2016"
	xos.CopyrightHolder = "Richard A. Wilkes"
	xos.License = "Mozilla Public License 2.0"

	var tmpl string
	commentStyle := single
	extMap := make(map[string]bool)
	quiet := false
	extensions := "go"
	years := fmt.Sprintf("%d", time.Now().Year())
	authors := os.Getenv("USER")
	if u, err := user.Current(); err == nil {
		authors = u.Name
	}
	xflag.SetUsage(nil, "Inserts and adjusts copyright notices in source files.", "<dir | file>...")
	xflag.AddVersionFlags()
	flag.StringVar(&tmpl, "template", "",
		"The template `file` to use for the copyright header. All occurrences of $YEAR$ within the template will be replaced with the current year. If this option is not specified, a default template will be used")
	flag.StringVar(&extensions, "extensions", extensions, "A comma-separated list of file `extensions` to process")
	flag.BoolVar(&quiet, "quiet", false, "Suppress progress messages")
	flag.StringVar(&commentStyle, "style", single,
		fmt.Sprintf("The style of comment to use for the copyright header. Possible `choice`s are '%s' for // ... comments, '%s' for /* ... */ comments, and '%s' for # ... comments", single, multi, hash))
	flag.StringVar(&years, "year", years,
		"One or more `years` to use in the copyright notice (e.g. '2025', '2019, 2025', '2019-2025', etc)")
	flag.StringVar(&authors, "author", authors,
		"The `names` of one or more authors to use in the copyright notice")
	xflag.Parse()
	targets := flag.Args()
	if len(targets) == 0 {
		xos.ExitWithMsg("at least one directory or file must be specified")
	}

	if commentStyle != single && commentStyle != multi && commentStyle != hash {
		xos.ExitWithMsg(fmt.Sprintf("style must be one of: %s, %s, %s", single, multi, hash))
	}

	for _, ext := range strings.Split(extensions, ",") {
		if ext != "" {
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			extMap[ext] = true
		}
	}
	if len(extMap) == 0 {
		xos.ExitWithMsg("extensions must specify at least one extension.")
	}

	if tmpl != "" {
		templateBytes, err := os.ReadFile(tmpl)
		xos.ExitIfErr(err)
		tmpl = string(templateBytes)
	} else {
		tmpl = `Copyright (c) {{.Years}} by {{.Authors}}. All rights reserved.

This Source Code Form is subject to the terms of the Mozilla Public
License, version 2.0. If a copy of the MPL was not distributed with
this file, You can obtain one at http://mozilla.org/MPL/2.0/.

This Source Code Form is "Incompatible With Secondary Licenses", as
defined by the Mozilla Public License, version 2.0.`
	}
	tmpl = processTemplate(tmpl, commentStyle, years, authors)
	for _, target := range targets {
		xos.ExitIfErr(filepath.Walk(target, func(path string, fi fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				path = filepath.Base(path)
				if path != "." && path != ".." && strings.HasPrefix(path, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if extMap[filepath.Ext(path)] {
				var buffer *bytes.Buffer
				if buffer, err = loadFile(path); err != nil {
					return err
				}
				var out *os.File
				if out, err = os.Create(path); err != nil {
					return err
				}
				defer xio.CloseLoggingErrors(out)
				if _, err = out.WriteString(tmpl); err != nil {
					return err
				}
				if buffer.Len() > 0 {
					data := buffer.Bytes()
					if data[0] != '\n' {
						if _, err = out.WriteString("\n"); err != nil {
							return err
						}
					}
					if _, err = buffer.WriteTo(out); err != nil {
						return errs.Wrap(err)
					}
				}
				if !quiet {
					fmt.Printf("Updated %s\n", path)
				}
			}
			return nil
		}))
	}
}

func processTemplate(tmpl, commentStyle, years, authors string) string {
	var buffer bytes.Buffer
	tmpl = strings.ReplaceAll(tmpl, "{{.Years}}", years)
	tmpl = strings.ReplaceAll(tmpl, "{{.Authors}}", authors)
	scanner := bufio.NewScanner(strings.NewReader(tmpl))
	var prefix string
	switch commentStyle {
	case multi:
		buffer.WriteString("/*\n")
		prefix = " *"
	case hash:
		prefix = "#"
	default: // single
		prefix = "//"
	}
	for scanner.Scan() {
		line := scanner.Text()
		buffer.WriteString(prefix)
		if line != "" {
			buffer.WriteString(" ")
			buffer.WriteString(line)
		}
		buffer.WriteString("\n")
	}
	if commentStyle == multi {
		buffer.WriteString(" */\n")
	}
	xos.ExitIfErr(scanner.Err())
	return buffer.String()
}

func loadFile(path string) (content *bytes.Buffer, err error) {
	var f *os.File
	if f, err = os.Open(path); err != nil {
		return nil, errs.Wrap(err)
	}
	defer xio.CloseIgnoringErrors(f)
	var contentBuffer, trimmedBuffer bytes.Buffer
	const (
		lookForAny = iota
		lookForSlashSlash
		lookForStarSlash
		lookForHash
		copyRemainder
	)
	state := lookForAny
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch state {
		case lookForAny:
			switch {
			case strings.HasPrefix(line, "//"):
				trimmedBuffer.WriteString(line)
				trimmedBuffer.WriteString("\n")
				state = lookForSlashSlash
			case strings.HasPrefix(line, "/*"):
				trimmedBuffer.WriteString(line)
				trimmedBuffer.WriteString("\n")
				if strings.HasSuffix(strings.TrimSpace(line), "*/") {
					state = copyRemainder
				} else {
					state = lookForStarSlash
				}
			case strings.HasPrefix(line, "#"):
				trimmedBuffer.WriteString(line)
				trimmedBuffer.WriteString("\n")
				state = lookForHash
			default:
				contentBuffer.WriteString(line)
				contentBuffer.WriteString("\n")
				state = copyRemainder
			}
		case lookForSlashSlash:
			if strings.HasPrefix(line, "//") {
				trimmedBuffer.WriteString(line)
				trimmedBuffer.WriteString("\n")
			} else {
				contentBuffer.WriteString(line)
				contentBuffer.WriteString("\n")
				state = copyRemainder
			}
		case lookForStarSlash:
			trimmedBuffer.WriteString(line)
			trimmedBuffer.WriteString("\n")
			if strings.HasSuffix(strings.TrimSpace(line), "*/") {
				state = copyRemainder
			}
		case lookForHash:
			if strings.HasPrefix(line, "#") {
				trimmedBuffer.WriteString(line)
				trimmedBuffer.WriteString("\n")
			} else {
				contentBuffer.WriteString(line)
				contentBuffer.WriteString("\n")
				state = copyRemainder
			}
		case copyRemainder:
			contentBuffer.WriteString(line)
			contentBuffer.WriteString("\n")
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, errs.Wrap(err)
	}
	trimmed := trimmedBuffer.Bytes()
	if !bytes.Contains(trimmed, []byte("opyright")) {
		trimmedBuffer.Write(contentBuffer.Bytes())
		return &trimmedBuffer, nil
	}
	return &contentBuffer, nil
}
