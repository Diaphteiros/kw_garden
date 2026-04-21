package version

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	buildVersion string
	gitTreeState string
	gitCommit    string
	buildDate    string
)

type Version struct {
	Version       string  `json:"version"`
	GitTreeState  string  `json:"gitTreeState"`
	GitCommit     string  `json:"gitCommit"`
	BuildDate     string  `json:"buildDate"`
	MajorVersion  *int    `json:"major,omitempty"`
	MinorVersion  *int    `json:"minor,omitempty"`
	PatchVersion  *int    `json:"patch,omitempty"`
	VersionSuffix *string `json:"suffix,omitempty"`
}

func Get() Version {
	res := Version{
		Version:      buildVersion,
		GitTreeState: gitTreeState,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
	}
	versionWithoutPrefix := strings.TrimPrefix(buildVersion, "v") // split off 'v' prefix, if any
	suffixSplit := strings.SplitN(versionWithoutPrefix, "-", 2)   // suffix is expected to start with '-'
	vSplit := strings.Split(suffixSplit[0], ".")
	if len(vSplit) >= 1 {
		if i, err := strconv.Atoi(vSplit[0]); err != nil {
			panic(invalidVersionError(buildVersion, fmt.Errorf("error converting major version '%s' to int: %w", vSplit[0], err)))
		} else {
			res.MajorVersion = &i
		}
	}
	if len(vSplit) >= 2 {
		if i, err := strconv.Atoi(vSplit[1]); err != nil {
			panic(invalidVersionError(buildVersion, fmt.Errorf("error converting minor version '%s' to int: %w", vSplit[1], err)))
		} else {
			res.MinorVersion = &i
		}
	}
	if len(vSplit) >= 3 {
		if i, err := strconv.Atoi(vSplit[2]); err != nil {
			panic(invalidVersionError(buildVersion, fmt.Errorf("error converting patch version '%s' to int: %w", vSplit[2], err)))
		} else {
			res.PatchVersion = &i
		}
	}
	if len(suffixSplit) == 2 {
		res.VersionSuffix = &suffixSplit[1]
	}
	return res
}

func invalidVersionError(v string, err error) error {
	return fmt.Errorf("invalid version: %s doesn't follow version format [v]<MAJOR>.<MINOR>.<PATCH>[-<SUFFIX>]: %w", v, err)
}

func (v *Version) String() string {
	return v.Version
}
