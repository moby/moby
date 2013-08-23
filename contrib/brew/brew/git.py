import tempfile
import logging

from dulwich import index
from dulwich.client import get_transport_and_path
from dulwich.repo import Repo

logger = logging.getLogger(__name__)


def clone_branch(repo_url, branch="master", folder=None):
    return clone(repo_url, 'refs/heads/' + branch, folder)


def clone_tag(repo_url, tag, folder=None):
    return clone(repo_url, 'refs/tags/' + tag, folder)


def checkout(rep, ref=None):
    is_commit = False
    if ref is None:
        ref = 'refs/heads/master'
    elif not ref.startswith('refs/'):
        is_commit = True
    if is_commit:
        rep['HEAD'] = rep.commit(ref)
    else:
        rep['HEAD'] = rep.refs[ref]
    indexfile = rep.index_path()
    tree = rep["HEAD"].tree
    index.build_index_from_tree(rep.path, indexfile, rep.object_store, tree)
    return rep.path

def clone(repo_url, ref=None, folder=None):
    is_commit = False
    if ref is None:
        ref = 'refs/heads/master'
    elif not ref.startswith('refs/'):
        is_commit = True
    logger.debug("clone repo_url={0}, ref={1}".format(repo_url, ref))
    if folder is None:
        folder = tempfile.mkdtemp()
    logger.debug("folder = {0}".format(folder))
    rep = Repo.init(folder)
    client, relative_path = get_transport_and_path(repo_url)
    logger.debug("client={0}".format(client))

    remote_refs = client.fetch(relative_path, rep)
    for k, v in remote_refs.iteritems():
        try:
            rep.refs.add_if_new(k, v)
        except:
            pass

    if is_commit:
        rep['HEAD'] = rep.commit(ref)
    else:
        rep['HEAD'] = remote_refs[ref]
    indexfile = rep.index_path()
    tree = rep["HEAD"].tree
    index.build_index_from_tree(rep.path, indexfile, rep.object_store, tree)
    logger.debug("done")
    return rep, folder
