// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package tools

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"launchpad.net/juju-core/environs"
	coretools "launchpad.net/juju-core/tools"
	"launchpad.net/juju-core/version"
)

var ErrNoTools = errors.New("no tools available")

const (
	DefaultToolPrefix = "tools/juju-"
	toolSuffix        = ".tgz"
)

var toolPrefix string = DefaultToolPrefix

// SetToolPrefix changes the prefix used to compose the tools tarball file name.
func SetToolPrefix(prefix string) {
	toolPrefix = prefix
}

// StorageName returns the name that is used to store and retrieve the
// given version of the juju tools.
func StorageName(vers version.Binary) string {
	return toolPrefix + vers.String() + toolSuffix
}

// ReadList returns a List of the tools in store with the given major.minor version.
// If minorVersion = -1, then only majorVersion is considered.
// If store contains no such tools, it returns ErrNoMatches.
func ReadList(storage environs.StorageReader, majorVersion, minorVersion int) (coretools.List, error) {
	logger.Debugf("reading v%d.%d tools", majorVersion, minorVersion)
	names, err := storage.List(toolPrefix)
	if err != nil {
		return nil, err
	}
	var list coretools.List
	var foundAnyTools bool
	for _, name := range names {
		if !strings.HasPrefix(name, toolPrefix) || !strings.HasSuffix(name, toolSuffix) {
			continue
		}
		var t coretools.Tools
		vers := name[len(toolPrefix) : len(name)-len(toolSuffix)]
		if t.Version, err = version.ParseBinary(vers); err != nil {
			continue
		}
		foundAnyTools = true
		// Major version must match specified value.
		if t.Version.Major != majorVersion {
			continue
		}
		// If specified minor version value supplied, minor version must match.
		if minorVersion >= 0 && t.Version.Minor != minorVersion {
			continue
		}
		logger.Debugf("found %s", vers)
		if t.URL, err = storage.URL(name); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	if len(list) == 0 {
		if foundAnyTools {
			return nil, coretools.ErrNoMatches
		}
		return nil, ErrNoTools
	}
	return list, nil
}

// Upload builds whatever version of launchpad.net/juju-core is in $GOPATH,
// uploads it to the given storage, and returns a Tools instance describing
// them. If forceVersion is not nil, the uploaded tools bundle will report
// the given version number; if any fakeSeries are supplied, additional copies
// of the built tools will be uploaded for use by machines of those series.
// Juju tools built for one series do not necessarily run on another, but this
// func exists only for development use cases.
func Upload(storage environs.Storage, forceVersion *version.Number, fakeSeries ...string) (*coretools.Tools, error) {
	// TODO(rog) find binaries from $PATH when not using a development
	// version of juju within a $GOPATH.

	logger.Debugf("Uploading tools for %v", fakeSeries)
	// We create the entire archive before asking the environment to
	// start uploading so that we can be sure we have archived
	// correctly.
	f, err := ioutil.TempFile("", "juju-tgz")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(f.Name())
	toolsVersion, err := bundleTools(f, forceVersion)
	if err != nil {
		return nil, err
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat newly made tools archive: %v", err)
	}
	size := fileInfo.Size()
	logger.Infof("built %v (%dkB)", toolsVersion, (size+512)/1024)
	putTools := func(vers version.Binary) (string, error) {
		if _, err := f.Seek(0, 0); err != nil {
			return "", fmt.Errorf("cannot seek to start of tools archive: %v", err)
		}
		name := StorageName(vers)
		logger.Infof("uploading %s", vers)
		if err := storage.Put(name, f, size); err != nil {
			return "", err
		}
		return name, nil
	}
	for _, series := range fakeSeries {
		if series != toolsVersion.Series {
			fakeVersion := toolsVersion
			fakeVersion.Series = series
			if _, err := putTools(fakeVersion); err != nil {
				return nil, err
			}
		}
	}
	name, err := putTools(toolsVersion)
	if err != nil {
		return nil, err
	}
	url, err := storage.URL(name)
	if err != nil {
		return nil, err
	}
	return &coretools.Tools{toolsVersion, url}, nil
}
