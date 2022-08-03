package istioctl

import (
	"os/exec"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
)

//Version Represents a specific istioctl executable version (e.g. 1.11.4)
type Version struct {
	value semver.Version
}

//VersionFromString returns a Version from passed semantic version in the format: "major.minor.patch", where all components must be positive integers
func VersionFromString(version string) (Version, error) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return Version{}, errors.New("invalid istioctl version format: empty input")
	}

	val, err := semver.NewVersion(trimmed)

	if err != nil {
		return Version{}, errors.Errorf("Invalid istioctl version format for input '%s': %s", trimmed, err.Error())
	}

	return Version{*val}, nil
}

func (v Version) String() string {
	return v.value.String()
}

func (v Version) SmallerThan(other Version) bool {
	return v.value.LessThan(other.value)
}

func (v Version) EqualTo(other Version) bool {
	return v.value.Equal(other.value)
}

func (v Version) BiggerThan(other Version) bool {
	return !(v.EqualTo(other) || v.SmallerThan(other))
}

//go:generate mockery --name=IstioctlResolver --outpkg=mock --case=underscore
// Finds an Executable for given Version
type ExecutableResolver interface {
	FindIstioctl(version Version) (*Executable, error)
}

type DefaultIstioctlResolver struct {
	sortedBinaries []Executable
}

func (d *DefaultIstioctlResolver) FindIstioctl(version Version) (*Executable, error) {
	return d.findMatchingBinary(version)
}

func NewDefaultIstioctlResolver(paths []string, vc VersionChecker) (*DefaultIstioctlResolver, error) {
	binariesList := []Executable{}
	for _, path := range paths {
		version, err := vc.GetIstioVersion(path)
		if err != nil {
			return nil, err
		}
		binariesList = append(binariesList, Executable{version, path})
	}

	sortBinaries(binariesList)

	return &DefaultIstioctlResolver{
		sortedBinaries: binariesList,
	}, nil
}

func (d *DefaultIstioctlResolver) findMatchingBinary(version Version) (*Executable, error) {
	matching := []Executable{}

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

//go:generate mockery --name=VersionChecker --output=istioctl --case=underscore
// VersionChecker implementations are able to return istioctl executable version
type VersionChecker interface {
	// GetIstioVersion return istioctl binary version given it's path
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

// Executable represents an istioctl executable in a specific version existing in a local filesystem
type Executable struct {
	version Version
	path    string
}

func (e Executable) SmallerThan(other Executable) bool {
	return e.version.SmallerThan(other.version)
}

func (e Executable) Path() string {
	return e.path
}

func (e Executable) Version() Version {
	return e.version
}

func sortBinaries(binaries []Executable) {
	sort.SliceStable(binaries, func(i, j int) bool {
		return binaries[i].SmallerThan(binaries[j])
	})
}
