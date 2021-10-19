package istioctl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type IstioVersion struct {
	major int
	minor int
	patch int
}

func istioVersionFromString(token string) (IstioVersion, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return IstioVersion{}, errors.New("Invalid istioctl version format: \"" + token + "\"")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return IstioVersion{}, errors.New("Invalid istioctl major version: \"" + parts[0] + "\"")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return IstioVersion{}, errors.New("Invalid istioctl minor version: \"" + parts[1] + "\"")
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return IstioVersion{}, errors.New("Invalid istioctl patch version: \"" + parts[2] + "\"")
	}

	return IstioVersion{major, minor, patch}, nil
}

func (v IstioVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (this IstioVersion) SmallerThan(other IstioVersion) bool {
	if this.major < other.major {
		return true
	} else if this.major > other.major {
		return false
	} else {
		if this.minor < other.minor {
			return true
		} else if this.minor > other.minor {
			return false
		} else {
			return this.patch < other.patch
		}
	}
}

func (this IstioVersion) BiggerThan(other IstioVersion) bool {
	return !(this.EqualTo(other) || this.SmallerThan(other))
}

func (this IstioVersion) EqualTo(other IstioVersion) bool {
	return this.major == other.major && this.minor == other.minor && this.patch == other.patch
}

//go:generate mockery --name=IstioctlResolver --outpkg=mock --case=underscore
// Provides a Commander for given istio version
type IstioctlResolver interface {
	GetCommander(version IstioVersion) (Commander, error)
}

type DefaultIstioctlResolver struct {
	sortedBinaries []istioctlBinary
}

func (d *DefaultIstioctlResolver) GetCommander(version IstioVersion) (Commander, error) {
	return nil, nil
}

type VersionChecker interface {
	GetIstioVersion(pathToBinary string) (IstioVersion, error)
}

func NewDefaultIstioctlResolver(paths []string, vc VersionChecker) (*DefaultIstioctlResolver, error) {
	binariesList := []istioctlBinary{}
	for _, path := range paths {
		version, err := vc.GetIstioVersion(path)
		if err != nil {
			return nil, err
		}
		binariesList = append(binariesList, istioctlBinary{version, path})
	}

	sortBinaries(binariesList)

	return &DefaultIstioctlResolver{
		sortedBinaries: binariesList,
	}, nil
}

func (d *DefaultIstioctlResolver) findMatchingBinary(version IstioVersion) (*istioctlBinary, error) {
	matching := []istioctlBinary{}

	for _, binary := range d.sortedBinaries {
		if binary.version.major == version.major && binary.version.minor == version.minor {
			matching = append(matching, binary)
		}
	}

	if len(matching) == 0 {
		return nil, errors.New("No matching Istioctl binary found for version: " + fmt.Sprintf("%s", version))
	}

	//Always return the biggest patch version from the available ones.
	return &matching[len(matching)-1], nil
}

// Default implementation that executes istioctl to find out it's version
type DefaultVersionChecker struct {
}

func (dvc DefaultVersionChecker) GetIstioVersion(pathToBinary string) (IstioVersion, error) {

	cmd := execCommand(pathToBinary, "version", "-s")
	out, err := cmd.Output()
	if err != nil {
		return IstioVersion{}, err
	}

	return istioVersionFromString(string(out))
}

// Represents a istioctl binary with a specific version
type istioctlBinary struct {
	version IstioVersion
	path    string
}

func (this istioctlBinary) SmallerThan(other istioctlBinary) bool {
	return this.version.SmallerThan(other.version)
}

func sortBinaries(binaries []istioctlBinary) {
	sort.SliceStable(binaries, func(i, j int) bool {
		return binaries[i].SmallerThan(binaries[j])
	})
}
