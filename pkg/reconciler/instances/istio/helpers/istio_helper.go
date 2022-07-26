package helpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
)

type HelperVersion struct {
	Library string
	Tag     semver.Version
}

func NewHelperVersionFrom(image string) (HelperVersion, error) {
	splitted := strings.Split(image, ":")
	if len(splitted) != 2 {
		return HelperVersion{}, errors.New("image doesn't contain repository and tag")
	}
	library := splitted[0]

	tag, err := semver.NewVersion(splitted[1])
	if err != nil {
		return HelperVersion{}, err
	}
	return HelperVersion{Library: library, Tag: *tag}, err
}

func (h HelperVersion) Compare(second HelperVersion) int {
	if h.Tag.Major > second.Tag.Major {
		return 1
	} else if h.Tag.Major == second.Tag.Major {
		if h.Tag.Minor > second.Tag.Minor {
			return 1
		} else if h.Tag.Minor == second.Tag.Minor {
			if h.Tag.Patch > second.Tag.Patch {
				return 1
			} else if h.Tag.Patch == second.Tag.Patch {
				return 0
			} else {
				return -1
			}
		} else {
			return -1
		}
	} else {
		return -1
	}
}

func (h HelperVersion) String() string {
	return fmt.Sprintf("%s:%s", h.Library, h.Tag.String())
}

func AreEqual(first, second HelperVersion) bool {
	return first.Library == second.Library && first.Tag.Equal(second.Tag)
}
