package packageregistry

import (
	"context"
	"fmt"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/google/go-github/v70/github"
	"github.com/safedep/dry/adapters"
)

type githubPackageRegistryAdapter struct{}
type githubPackageRegistryPublisherDiscovery struct{}
type githubPackageRegistryPackageDiscovery struct{}

// Verify that githubPackageRegistryAdapter implements the Client interface
var _ Client = (*githubPackageRegistryAdapter)(nil)

// NewGithubPackageRegistryAdapter creates a new GitHub package registry adapter
func NewGithubPackageRegistryAdapter() (Client, error) {
	return &githubPackageRegistryAdapter{}, nil
}

func (ga *githubPackageRegistryAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &githubPackageRegistryPublisherDiscovery{}, nil
}

func (ga *githubPackageRegistryAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &githubPackageRegistryPackageDiscovery{}, nil
}

func (ga *githubPackageRegistryPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	ctx := context.Background()

	pkgName := packageVersion.Package.GetName()

	ghClient, err := adapters.NewGithubClient(adapters.DefaultGitHubClientConfig())
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	tokens := strings.Split(pkgName, "/")
	if len(tokens) != 2 {
		return nil, ErrNoPackagesFound
	}

	owner := tokens[0]
	repo := tokens[1]

	repository, _, err := ghClient.Client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	publisher := []Publisher{}

	publisher = append(publisher, Publisher{
		Name:  repository.GetOwner().GetLogin(),
		Email: repository.GetOwner().GetEmail(),
		Url:   fmt.Sprintf("https://github.com/%s", owner),
	})

	return &PackagePublisherInfo{
		Publishers: publisher,
	}, nil
}

func (ga *githubPackageRegistryPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	ctx := context.Background()

	ghClient, err := adapters.NewGithubClient(adapters.DefaultGitHubClientConfig())
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	repos, _, err := ghClient.Client.Repositories.ListByUser(ctx, publisher.Name, nil)
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	packages := []*Package{}

	for _, repo := range repos {
		latestVersion := getGitHubRepositoryLatestVersion(ctx, ghClient, repo)
		pkgVersions, err := getGitHubRepositoryVersions(ctx, ghClient, repo)
		if err != nil {
			return nil, err
		}

		pkg := githubRegistryCreatePackageWrapper(repo, latestVersion, pkgVersions)
		packages = append(packages, pkg)
	}

	return packages, nil
}

// GetPackage returns the package details from the package name
// For GitHub the package name is the {owner}/{repo}
func (ga *githubPackageRegistryPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	ctx := context.Background()

	ghClient, err := adapters.NewGithubClient(adapters.DefaultGitHubClientConfig())
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	tokens := strings.Split(packageName, "/")
	if len(tokens) != 2 {
		return nil, ErrNoPackagesFound
	}

	owner := tokens[0]
	repo := tokens[1]

	repository, _, err := ghClient.Client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	latestVersion := getGitHubRepositoryLatestVersion(ctx, ghClient, repository)

	pkgVersions, err := getGitHubRepositoryVersions(ctx, ghClient, repository)
	if err != nil {
		return nil, err
	}

	return githubRegistryCreatePackageWrapper(repository, latestVersion, pkgVersions), nil
}

// getGitHubRepositoryLatestVersion returns the latest version of the repository
// If there is no release, it returns the default branch
func getGitHubRepositoryLatestVersion(ctx context.Context, ghClient *adapters.GithubClient, repo *github.Repository) string {
	latestRelease, _, _ := ghClient.Client.Repositories.GetLatestRelease(ctx, repo.GetOwner().GetLogin(), repo.GetName())

	if latestRelease != nil && latestRelease.GetTagName() != "" {
		return latestRelease.GetTagName()
	}
	return repo.GetDefaultBranch()
}

// getGitHubRepositoryVersions returns all versions of the repository
func getGitHubRepositoryVersions(ctx context.Context, ghClient *adapters.GithubClient, repo *github.Repository) ([]PackageVersionInfo, error) {
	pkgVersions := []PackageVersionInfo{}

	releases, _, err := ghClient.Client.Repositories.ListReleases(ctx, repo.GetOwner().GetLogin(), repo.GetName(), nil)
	if err != nil {
		return nil, ErrNoPackagesFound
	}

	for _, release := range releases {
		pkgVersions = append(pkgVersions, PackageVersionInfo{
			Version: release.GetTagName(),
		})
	}

	return pkgVersions, nil
}

// githubRegistryCreatePackageWrapper creates a new package wrapper form github.Repository
func githubRegistryCreatePackageWrapper(repo *github.Repository, latestVersion string, pkgVersions []PackageVersionInfo) *Package {
	pkg := &Package{
		Name:                repo.GetFullName(),
		Description:         repo.GetDescription(),
		SourceRepositoryUrl: fmt.Sprintf("https://github.com/%s/%s", repo.GetOwner().GetLogin(), repo.GetName()),
		Author: Publisher{
			Name:  repo.GetOwner().GetLogin(),
			Email: repo.GetOwner().GetEmail(),
			Url:   repo.GetOwner().GetURL(),
		},
		LatestVersion: latestVersion,
		Versions:      pkgVersions,
		CreatedAt:     repo.GetCreatedAt().Time,
		UpdatedAt:     repo.GetUpdatedAt().Time,
	}

	return pkg
}
