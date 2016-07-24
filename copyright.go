// Copyright (c) 2016 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// copyright inserts and adjusts copyright notices in source files.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/richardwilkes/cmdline"
	"github.com/richardwilkes/errors"
	"github.com/richardwilkes/i18n"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	single = "single"
	multi  = "multi"
	hash   = "hash"
)

var (
	version      string
	cl           *cmdline.CmdLine
	commentStyle = single
	extMap       = make(map[string]bool)
	template     string
	quiet        bool
)

func main() {
	errors.Detailed = false
	cmdline.AppVersion = cmdline.NewVersionFromString(version)
	cmdline.CopyrightYears = "2016"
	cmdline.CopyrightHolder = "Richard A. Wilkes"
	cmdline.License = "Mozilla Public License 2.0"

	var (
		extensions = "go"
		year       = fmt.Sprintf("%d", time.Now().Year())
	)
	cl = cmdline.New(true)
	cl.NewStringOption(&template).SetName("template").SetSingle('t').SetArg(i18n.Text("file")).SetUsage(i18n.Text("The template to use for the copyright header. All occurrences of $YEAR$ within the template will be replaced with the current year. If this option is not specified, a default template will be used")).SetDefault("")
	cl.NewStringOption(&extensions).SetName("extensions").SetSingle('e').SetUsage(i18n.Text("A comma-separated list of file extensions to process"))
	cl.NewBoolOption(&errors.Detailed).SetName("debug").SetSingle('d').SetUsage(i18n.Text("Enable debugging output"))
	cl.NewBoolOption(&quiet).SetName("quiet").SetSingle('q').SetUsage(i18n.Text("Suppress progress messages"))
	cl.NewStringOption(&commentStyle).SetName("style").SetSingle('s').SetUsage(fmt.Sprintf(i18n.Text("The style of comment to use for the copyright header. Choices are '%s' for // ... comments, '%s' for /* ... */ comments, and '%s' for # ... comments"), single, multi, hash))
	cl.NewStringOption(&year).SetName("year").SetSingle('y').SetUsage(i18n.Text("The year(s) to use in the copyright notice"))
	cl.UsageSuffix = i18n.Text("<dir | file>...")
	cl.Description = i18n.Text("Inserts and adjusts copyright notices in source files.")
	targets := cl.Parse(os.Args[1:])

	if len(targets) == 0 {
		cl.FatalMsg(i18n.Text("At least one directory or file must be specified."))
	}

	if commentStyle != single && commentStyle != multi && commentStyle != hash {
		cl.FatalMsg(fmt.Sprintf(i18n.Text("The style option must be one of: %s, %s, %s"), single, multi, hash))
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
		cl.FatalMsg(i18n.Text("The extensions option must specify at least one extension."))
	}

	if template != "" {
		template = loadTemplate()
	} else {
		template = `Copyright (c) $YEAR$ by Richard A. Wilkes. All rights reserved.

This Source Code Form is subject to the terms of the Mozilla Public
License, version 2.0. If a copy of the MPL was not distributed with
this file, You can obtain one at http://mozilla.org/MPL/2.0/.

This Source Code Form is "Incompatible With Secondary Licenses", as
defined by the Mozilla Public License, version 2.0.`
	}
	template = processTemplate(year)

	for _, target := range targets {
		if err := filepath.Walk(target, processFile); err != nil {
			cl.FatalError(errors.Wrap(err))
		}
	}
}

func loadTemplate() string {
	var file *os.File
	var err error
	if file, err = os.Open(template); err != nil {
		cl.FatalError(errors.NewWithCause(i18n.Text("Unable to open the template file."), err))
	}
	defer file.Close()
	var fi os.FileInfo
	if fi, err = file.Stat(); err != nil {
		cl.FatalError(errors.Wrap(err))
	}
	if fi.IsDir() {
		cl.FatalMsg(i18n.Text("The template must be a file."))
	}
	buffer := make([]byte, fi.Size())
	var read int
	if read, err = file.Read(buffer); err != nil || read != len(buffer) {
		cl.FatalError(errors.NewWithCause(i18n.Text("Unable to read template file."), err))
	}
	return string(buffer)
}

func processTemplate(year string) string {
	var buffer bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(strings.Replace(template, "$YEAR$", year, -1)))
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
	if err := scanner.Err(); err != nil {
		cl.FatalError(errors.NewWithCause(i18n.Text("Unable to process template."), err))
	}
	return buffer.String()
}

func processFile(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return errors.Wrap(err)
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
			return errors.Wrap(err)
		}
		var out *os.File
		if out, err = os.Create(path); err != nil {
			return errors.Wrap(err)
		}
		defer out.Close()
		if _, err = out.WriteString(template); err != nil {
			return errors.Wrap(err)
		}
		if buffer.Len() > 0 {
			bytes := buffer.Bytes()
			if bytes[0] != '\n' {
				if _, err = out.WriteString("\n"); err != nil {
					return errors.Wrap(err)
				}
			}
			if _, err = buffer.WriteTo(out); err != nil {
				return errors.Wrap(err)
			}
		}
		if !quiet {
			fmt.Printf(i18n.Text("Updated %s\n"), path)
		}
	}
	return nil
}

func loadFile(path string) (content *bytes.Buffer, err error) {
	var file *os.File
	if file, err = os.Open(path); err != nil {
		return nil, errors.Wrap(err)
	}
	defer file.Close()
	var buffer bytes.Buffer
	const (
		lookForSlashSlash = iota
		lookForSlashStar
		lookForStarSlash
		lookForHash
		copyRemainder
	)
	var state int
	switch commentStyle {
	case multi:
		state = lookForSlashStar
	case hash:
		state = lookForHash
	default:
		state = lookForSlashSlash
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch state {
		case lookForSlashSlash:
			if !strings.HasPrefix(line, "//") {
				buffer.WriteString(line)
				buffer.WriteString("\n")
				state = copyRemainder
			}
		case lookForSlashStar:
			if strings.HasPrefix(line, "/*") {
				state = lookForStarSlash
			} else {
				state = copyRemainder
			}
		case lookForStarSlash:
			if strings.HasSuffix(strings.TrimSpace(line), "*/") {
				state = copyRemainder
			}
		case lookForHash:
			if !strings.HasPrefix(line, "#") {
				buffer.WriteString(line)
				buffer.WriteString("\n")
				state = copyRemainder
			}
		case copyRemainder:
			buffer.WriteString(line)
			buffer.WriteString("\n")
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, errors.Wrap(err)
	}
	return &buffer, nil
}
