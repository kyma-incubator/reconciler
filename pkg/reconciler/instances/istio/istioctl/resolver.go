package istioctl

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
)

const (
	versionSuffixSeparator = "-"
)

type Version struct {
	value semver.Version
}

//Returns a Version from passed semantic version in the format: "major.minor.patch", where all components must be positive integers
func VersionFromString(version string) (Version, error) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return Version{}, errors.New("Invalid istioctl version format: empty input.")
	}

	val, err := semver.NewVersion(trimmed)

	if err != nil {
		return Version{}, errors.Errorf("Invalid istioctl version format for input '%s': %s", trimmed, err.Error())
	}

	return Version{*val}, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%s", v.value)
}

func (this Version) SmallerThan(other Version) bool {
	return this.value.LessThan(other.value)
}

func (this Version) EqualTo(other Version) bool {
	return this.value.Equal(other.value)
}

func (this Version) BiggerThan(other Version) bool {
	return !(this.EqualTo(other) || this.SmallerThan(other))
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

		if binary.version.value.Major == version.value.Major && binary.version.value.Minor == version.value.Minor {
			matching = append(matching, binary)
		}
	}

	if len(matching) == 0 {
		availableBinaries := []string{}
		for _, binary := range d.sortedBinaries {
			availableBinaries = append(availableBinaries, binary.Version().String())
		}
		versionList := strings.Join(availableBinaries, ", ")
		return nil, errors.Errorf("No matching 'istioctl' binary found for version: %s. Available binaries: %s", version.String(), versionList)
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

	cmd := exec.Command(pathToBinary, "version", "-s", "--remote=false")
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
