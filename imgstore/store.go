package imgstore

type Store interface {
	View

	Install(newimg *Image) (oldimg *Image, err error)
	Uninstall(repo string, tag string) (uninstalled View, err error)
}

type View interface {
	ListRepos() ([]string, error)
	ListTags(repo string) ([]string, error)
	Get(repo, tag string) (*Image, error)
	All() ([]*Image, error)

	RepoEquals(string) View
	TagEquals(string) View
	RepoLike(string) View
	TagLike(string) View
	TagGT(string) View
	TagLT(string) View
}
