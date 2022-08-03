package istioctl

import (
	"os/exec"
	"sort"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
	"github.com/pkg/errors"
)

//go:generate mockery --name=IstioctlResolver --outpkg=mock --case=underscore
// ExecutableResolver finds an Executable for given Version
type ExecutableResolver interface {
	FindIstioctl(version helpers.HelperVersion) (*Executable, error)
}

type DefaultIstioctlResolver struct {
	sortedBinaries []Executable
}

func (d *DefaultIstioctlResolver) FindIstioctl(version helpers.HelperVersion) (*Executable, error) {
	return d.findMatchingBinary(version)
}

func NewDefaultIstioctlResolver(paths []string, vc VersionChecker) (*DefaultIstioctlResolver, error) {
	var binariesList []Executable
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

func (d *DefaultIstioctlResolver) findMatchingBinary(version helpers.HelperVersion) (*Executable, error) {
	var matching []Executable

	for _, binary := range d.sortedBinaries {
		if helpers.AreEqual(binary.version, version) {
			return &binary, nil
		}

		if binary.version.Tag.Major == version.Tag.Major && binary.version.Tag.Minor == version.Tag.Minor {
			matching = append(matching, binary)
		}
	}

	if len(matching) == 0 {
		var availableBinaries []string
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
	GetIstioVersion(pathToBinary string) (helpers.HelperVersion, error)
}

// DefaultVersionChecker is the default implementation that executes istioctl to find out it's version
type DefaultVersionChecker struct {
}

func (dvc DefaultVersionChecker) GetIstioVersion(pathToBinary string) (helpers.HelperVersion, error) {

	cmd := exec.Command(pathToBinary, "version", "-s", "--remote=false")
	out, err := cmd.Output()
	if err != nil {
		return helpers.HelperVersion{}, err
	}

	ver, err := helpers.NewHelperVersionFrom(string(out))
	if err != nil {
		return helpers.HelperVersion{}, err
	}
	return *ver, nil
}

// Executable represents an istioctl executable in a specific version existing in a local filesystem
type Executable struct {
	version helpers.HelperVersion
	path    string
}

func (e Executable) SmallerThan(other Executable) bool {
	return e.version.Tag.LessThan(other.version.Tag)
}

func (e Executable) Path() string {
	return e.path
}

func (e Executable) Version() helpers.HelperVersion {
	return e.version
}

func sortBinaries(binaries []Executable) {
	sort.SliceStable(binaries, func(i, j int) bool {
		return binaries[i].SmallerThan(binaries[j])
	})
}
