package istioctl

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Version struct {
	major int
	minor int
	patch int
}

//Returns a Version from passed semantic version in the format: "major.minor.patch", where all components must be positive integers
func VersionFromString(semver string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(semver), ".")
	if len(parts) != 3 {
		return Version{}, errors.New("Invalid istioctl version format: \"" + semver + "\"")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, errors.New("Invalid istioctl major version: \"" + parts[0] + "\"")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, errors.New("Invalid istioctl minor version: \"" + parts[1] + "\"")
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, errors.New("Invalid istioctl patch version: \"" + parts[2] + "\"")
	}

	return Version{major, minor, patch}, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (this Version) SmallerThan(other Version) bool {
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

func (this Version) BiggerThan(other Version) bool {
	return !(this.EqualTo(other) || this.SmallerThan(other))
}

func (this Version) EqualTo(other Version) bool {
	return this.major == other.major && this.minor == other.minor && this.patch == other.patch
}

//go:generate mockery --name=IstioctlResolver --outpkg=mock --case=underscore
// Finds IstioctlBinary for given Version
type IstioctlResolver interface {
	FindIstioctl(version Version) (*IstioctlBinary, error)
}

type DefaultIstioctlResolver struct {
	sortedBinaries []IstioctlBinary
}

func (d *DefaultIstioctlResolver) FindIstioctl(version Version) (*IstioctlBinary, error) {
	return d.findMatchingBinary(version)
}

func NewDefaultIstioctlResolver(paths []string, vc VersionChecker) (*DefaultIstioctlResolver, error) {
	binariesList := []IstioctlBinary{}
	for _, path := range paths {
		version, err := vc.GetIstioVersion(path)
		if err != nil {
			return nil, err
		}
		binariesList = append(binariesList, IstioctlBinary{version, path})
	}

	sortBinaries(binariesList)

	return &DefaultIstioctlResolver{
		sortedBinaries: binariesList,
	}, nil
}

func (d *DefaultIstioctlResolver) findMatchingBinary(version Version) (*IstioctlBinary, error) {
	matching := []IstioctlBinary{}

	for _, binary := range d.sortedBinaries {
		if binary.version.EqualTo(version) {
			return &binary, nil
		}

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

//go:generate mockery --name=VersionChecker --outpkg=istioctl --case=underscore
type VersionChecker interface {
	GetIstioVersion(pathToBinary string) (Version, error)
}

// Default implementation that executes istioctl to find out it's version
type DefaultVersionChecker struct {
}

func (dvc DefaultVersionChecker) GetIstioVersion(pathToBinary string) (Version, error) {

	cmd := exec.Command(pathToBinary, "version", "-s")
	out, err := cmd.Output()
	if err != nil {
		return Version{}, err
	}

	return VersionFromString(string(out))
}

// Represents a istioctl binary with a specific version
type IstioctlBinary struct {
	version Version
	path    string
}

func (this IstioctlBinary) SmallerThan(other IstioctlBinary) bool {
	return this.version.SmallerThan(other.version)
}

func (this IstioctlBinary) Path() string {
	return this.path
}

func (this IstioctlBinary) Version() Version {
	return this.version
}

func sortBinaries(binaries []IstioctlBinary) {
	sort.SliceStable(binaries, func(i, j int) bool {
		return binaries[i].SmallerThan(binaries[j])
	})
}
