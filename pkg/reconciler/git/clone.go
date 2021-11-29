package git

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/go-git/go-git/v5"
	gitp "github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
)

type Cloner struct {
	repo         *reconciler.Repository
	autoCheckout bool

	repoClient         RepoClient
	inClusterClientSet k8s.Interface
	logger             *zap.SugaredLogger
}

//go:generate mockery --name RepoClient --case=underscore
type RepoClient interface {
	Clone(ctx context.Context, path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
	Worktree() (*git.Worktree, error)
	ResolveRevisionOrBranchHead(rev gitp.Revision) (*gitp.Hash, error)
	Fetch(path string, o *git.FetchOptions) error
	Repo() *git.Repository
	PlainCheckout(path string, o *git.CheckoutOptions) error
}

func NewCloner(repoClient RepoClient, repo *reconciler.Repository, autoCheckout bool, clientSet k8s.Interface, logger *zap.SugaredLogger) (*Cloner, error) {
	return &Cloner{
		repo:               repo,
		autoCheckout:       autoCheckout,
		repoClient:         repoClient,
		inClusterClientSet: clientSet,
		logger:             logger,
	}, nil
}

// Clone clones the repository from the given remote URL to the given `path` in the local filesystem.
func (r *Cloner) Clone(path string) (*git.Repository, error) {
	auth, err := r.buildAuth()
	if err != nil {
		return nil, err
	}
	return r.repoClient.Clone(context.Background(), path, false, &git.CloneOptions{
		Depth:             0,
		URL:               r.repo.URL,
		NoCheckout:        !r.autoCheckout,
		Auth:              auth,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})

}

// Checkout checks out the given revision.
// revision can be 'main', a release version (e.g. 1.4.1), a commit hash (e.g. 34edf09a).
func (r *Cloner) Checkout(rev string, repo *git.Repository) error {
	w, err := r.repoClient.Worktree()
	if err != nil {
		return errors.Wrap(err, "error getting the GIT worktree")
	}

	// hash, err := r.repoClient.ResolveRevision(gitp.Revision(rev))
	var defaultLister refLister = remoteRefLister{}
	var resolver = revisionResolver{url: r.repo.URL, repository: repo, refLister: defaultLister}

	hash, err := resolver.resolveRevision(rev)
	if err != nil {
		msg := fmt.Sprintf("failed to resolve GIT revision '%s'", rev)
		if r.repo.URL != "" {
			msg += fmt.Sprintf(" using repository '%s' ",
				r.repo.URL)
		}
		return errors.Wrap(err, msg)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})

	if err != nil {
		return errors.Wrap(err, "Error checking out GIT revision")
	}
	return nil
}

func (r *Cloner) CloneAndCheckout(dstPath, rev string) error {
	repo, err := r.Clone(dstPath)
	if err != nil {
		return errors.Wrapf(err, "Error downloading Git repository (%s)", r.repo)
	}
	if rev == "" {
		head, err := repo.Head()
		if err != nil {
			return err
		}
		rev = head.Hash().String()
	}
	return r.Checkout(rev, repo)
}

func (r *Cloner) buildAuth() (transport.AuthMethod, error) {
	tokenNamespace := "default"
	if r.repo.TokenNamespace != "" {
		tokenNamespace = r.repo.TokenNamespace
	}

	if r.inClusterClientSet == nil {
		return nil, nil
	}

	secretKey, err := mapSecretKey(r.repo.URL)
	if err != nil {
		return nil, err
	}

	secret, err := r.inClusterClientSet.CoreV1().
		Secrets(tokenNamespace).
		Get(context.Background(), secretKey, v1.GetOptions{})

	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) {
		return nil, err
	}

	if secret != nil && err == nil {
		return &http.BasicAuth{
			Username: "xxx", // anything but an empty string
			Password: strings.Trim(string(secret.Data["token"]), "\n"),
		}, nil
	}

	r.logger.Info("Token not found or forbidden")

	return nil, nil
}

func mapSecretKey(URL string) (string, error) {
	if !strings.HasPrefix(URL, "http") {
		URL = "https://" + URL
	}

	URL = strings.ReplaceAll(URL, "www.", "")

	parsed, err := url.Parse(URL)

	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		return parsed.Path, nil
	}

	output := strings.ReplaceAll(parsed.Host, ":"+parsed.Port(), "")
	output = strings.ReplaceAll(output, "www.", "")

	return output, nil
}

func (r *Cloner) FetchAndCheckout(path, version string) error {
	auth, err := r.buildAuth()
	if err != nil {
		return err
	}
	err = r.repoClient.Fetch(path, &git.FetchOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{config.RefSpec("+refs/heads/*:refs/remotes/origin/*")},
	})
	if err != nil {
		return err
	}

	if version != "" {
		hash, err := r.ResolveRevisionOrBranchHead(version)
		if err != nil {
			return err
		}
		return r.repoClient.PlainCheckout(path, &git.CheckoutOptions{
			Hash: *hash,
		})
	}
	return nil
}

func (r *Cloner) ResolveRevisionOrBranchHead(rev string) (*gitp.Hash, error) {
	return r.repoClient.ResolveRevisionOrBranchHead(gitp.Revision(fmt.Sprintf(rev)))
}
