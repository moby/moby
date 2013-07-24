import os
import logging

import docker

import git

DEFAULT_REPOSITORY = 'git://github.com/dotcloud/docker'
DEFAULT_BRANCH = 'library'

logger = logging.getLogger(__name__)
logging.basicConfig(format='%(asctime)s %(levelname)s %(message)s',
                    level='INFO')
client = docker.Client()
processed = {}


def build_library(repository=None, branch=None, namespace=None, push=False,
        debug=False):
    if repository is None:
        repository = DEFAULT_REPOSITORY
    if branch is None:
        branch = DEFAULT_BRANCH
    if debug:
        logger.setLevel('DEBUG')

    if not (repository.startswith('https://') or repository.startswith('git://')):
        logger.info('Repository provided assumed to be a local path')
        dst_folder = repository

    #FIXME: set destination folder and only pull latest changes instead of
    # cloning the whole repo everytime
    if not dst_folder:
        logger.info('Cloning docker repo from {0}, branch: {1}'.format(
            repository, branch))
        dst_folder = git.clone_branch(repository, branch)
    for buildfile in os.listdir(os.path.join(dst_folder, 'library')):
        if buildfile == 'MAINTAINERS':
            continue
        f = open(os.path.join(dst_folder, 'library', buildfile))
        for line in f:
            logger.debug('{0} ---> {1}'.format(buildfile, line))
            args = line.split()
            try:
                if len(args) > 3:
                    raise RuntimeError('Incorrect line format, '
                        'please refer to the docs')

                url = None
                ref = 'refs/heads/master'
                tag = None
                if len(args) == 1:  # Just a URL, simple mode
                    url = args[0]
                elif len(args) == 2 or len(args) == 3:  # docker-tag   url
                    url = args[1]
                    tag = args[0]

                if len(args) == 3:  # docker-tag  url     B:branch or T:tag
                    ref = None
                    if args[2].startswith('B:'):
                        ref = 'refs/heads/' + args[2][2:]
                    elif args[2].startswith('T:'):
                        ref = 'refs/tags/' + args[2][2:]
                    elif args[2].startswith('C:'):
                        ref = args[2][2:]
                    else:
                        raise RuntimeError('Incorrect line format, '
                            'please refer to the docs')
                img = build_repo(url, ref, buildfile, tag, namespace, push)
                processed['{0}@{1}'.format(url, ref)] = img
            except Exception as e:
                logger.exception(e)
        f.close()


def build_repo(repository, ref, docker_repo, docker_tag, namespace, push):
    docker_repo = '{0}/{1}'.format(namespace or 'library', docker_repo)
    img_id = None
    if '{0}@{1}'.format(repository, ref) not in processed.keys():
        logger.info('Cloning {0} (ref: {1})'.format(repository, ref))
        dst_folder = git.clone(repository, ref)
        if not 'Dockerfile' in os.listdir(dst_folder):
            raise RuntimeError('Dockerfile not found in cloned repository')
        logger.info('Building using dockerfile...')
        img_id, logs = client.build(path=dst_folder)

    if not img_id:
        img_id = processed['{0}@{1}'.format(repository, ref)]
    logger.info('Committing to {0}:{1}'.format(docker_repo,
        docker_tag or 'latest'))
    client.tag(img_id, docker_repo, docker_tag)
    if push:
        logger.info('Pushing result to the main registry')
        client.push(docker_repo)
    return img_id
