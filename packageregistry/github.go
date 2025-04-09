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
		pkg, err := getGitHubRepoDetails(ctx, ghClient, repo)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

// GetPackage returns the package details form the package nam
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

	return getGitHubRepoDetails(ctx, ghClient, repository)
}

func getGitHubRepoDetails(ctx context.Context, ghClient *adapters.GithubClient, repo *github.Repository) (*Package, error) {
	// Default fallback for latest version, to default branch (works good)
	latestVersion := repo.GetDefaultBranch()

	// We are not handling error since we anyway using default fallback
	latestRelease, _, _ := ghClient.Client.Repositories.GetLatestRelease(ctx, repo.GetOwner().GetLogin(), repo.GetName())
	if latestRelease != nil && latestRelease.GetTagName() != "" {
		latestVersion = latestRelease.GetTagName()
	}

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

	return pkg, nil
}
