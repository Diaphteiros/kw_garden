package version

import (
	"fmt"
	"strconv"
	"strings"
)

var Version *versionInformation

func init() {
	var err error
	Version, err = ParseVersion(StaticVersion)
	if err != nil {
		panic(fmt.Errorf("error parsing StaticVersion: %w", err))
	}
}

const (
	versionFormat = "[v]<MAJOR>.<MINOR>.<PATCH>[-<SUFFIX>]"
)

func invalidVersionError(v string) error {
	return fmt.Errorf("invalid version: %s doesn't follow version format %s", v, versionFormat)
}

type versionInformation struct {
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
	Patch   int    `json:"patch"`
	Suffix  string `json:"suffix,omitempty"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

func (v *versionInformation) String() string {
	return v.Version
}

// ParseVersion parses a string into a versionInformation.
func ParseVersion(v string) (*versionInformation, error) {
	ver := strings.TrimPrefix(v, "v") // split off 'v' prefix, if any
	vs := strings.SplitN(ver, "-", 2) // suffix is expected to start with '-'
	ver = vs[0]
	res := &versionInformation{}
	suffix := ""
	if len(vs) > 1 {
		res.Suffix = vs[1]
		suffix = fmt.Sprintf("-%s", res.Suffix)
		if strings.HasPrefix(res.Suffix, "dev-") {
			res.Commit = strings.TrimPrefix(res.Suffix, "dev-")
		}
	}
	vs = strings.Split(ver, ".")
	if len(vs) != 3 {
		return nil, invalidVersionError(v)
	}
	var err error
	res.Major, err = strconv.Atoi(vs[0])
	if err != nil {
		return nil, invalidVersionError(v)
	}
	res.Minor, err = strconv.Atoi(vs[1])
	if err != nil {
		return nil, invalidVersionError(v)
	}
	res.Patch, err = strconv.Atoi(vs[2])
	if err != nil {
		return nil, invalidVersionError(v)
	}

	res.Version = fmt.Sprintf("v%d.%d.%d%s", res.Major, res.Minor, res.Patch, suffix)

	return res, nil
}
