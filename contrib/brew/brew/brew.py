import os
import logging
from shutil import rmtree

import docker

import git

DEFAULT_REPOSITORY = 'git://github.com/dotcloud/docker'
DEFAULT_BRANCH = 'master'

logger = logging.getLogger(__name__)
logging.basicConfig(format='%(asctime)s %(levelname)s %(message)s',
                    level='INFO')
client = docker.Client()
processed = {}
processed_folders = []


def build_library(repository=None, branch=None, namespace=None, push=False,
        debug=False, prefill=True, registry=None):
    dst_folder = None
    summary = Summary()
    if repository is None:
        repository = DEFAULT_REPOSITORY
    if branch is None:
        branch = DEFAULT_BRANCH
    if debug:
        logger.setLevel('DEBUG')

    if not (repository.startswith('https://') or repository.startswith('git://')):
        logger.info('Repository provided assumed to be a local path')
        dst_folder = repository

    try:
        client.version()
    except Exception as e:
        logger.error('Could not reach the docker daemon. Please make sure it '
            'is running.')
        logger.warning('Also make sure you have access to the docker UNIX '
            'socket (use sudo)')
        return

    #FIXME: set destination folder and only pull latest changes instead of
    # cloning the whole repo everytime
    if not dst_folder:
        logger.info('Cloning docker repo from {0}, branch: {1}'.format(
            repository, branch))
        try:
            dst_folder = git.clone_branch(repository, branch)
        except Exception as e:
            logger.exception(e)
            logger.error('Source repository could not be fetched. Check '
                'that the address is correct and the branch exists.')
            return
    try:
        dirlist = os.listdir(os.path.join(dst_folder, 'library'))
    except OSError as e:
        logger.error('The path provided ({0}) could not be found or didn\'t'
            'contain a library/ folder.'.format(dst_folder))
        return
    for buildfile in dirlist:
        if buildfile == 'MAINTAINERS':
            continue
        f = open(os.path.join(dst_folder, 'library', buildfile))
        linecnt = 0
        for line in f:
            linecnt = linecnt + 1
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
                if prefill:
                    logger.debug('Pulling {0} from official repository (cache '
                        'fill)'.format(buildfile))
                    client.pull(buildfile)
                img = build_repo(url, ref, buildfile, tag, namespace, push,
                    registry)
                summary.add_success(buildfile, (linecnt, line), img)
                processed['{0}@{1}'.format(url, ref)] = img
            except Exception as e:
                logger.exception(e)
                summary.add_exception(buildfile, (linecnt, line), e)

        f.close()
    if dst_folder != repository:
        rmtree(dst_folder, True)
    for d in processed_folders:
        rmtree(d, True)
    summary.print_summary(logger)


def build_repo(repository, ref, docker_repo, docker_tag, namespace, push, registry):
    docker_repo = '{0}/{1}'.format(namespace or 'library', docker_repo)
    img_id = None
    dst_folder = None
    if '{0}@{1}'.format(repository, ref) not in processed.keys():
        logger.info('Cloning {0} (ref: {1})'.format(repository, ref))
        if repository not in processed:
            rep, dst_folder = git.clone(repository, ref)
            processed[repository] = rep
            processed_folders.append(dst_folder)
        else:
            dst_folder = git.checkout(processed[repository], ref)
        if not 'Dockerfile' in os.listdir(dst_folder):
            raise RuntimeError('Dockerfile not found in cloned repository')
        logger.info('Building using dockerfile...')
        img_id, logs = client.build(path=dst_folder, quiet=True)
    else:
        img_id = processed['{0}@{1}'.format(repository, ref)]
    logger.info('Committing to {0}:{1}'.format(docker_repo,
        docker_tag or 'latest'))
    client.tag(img_id, docker_repo, docker_tag)
    if push:
        logger.info('Pushing result to registry {0}'.format(
            registry or "default"))
        if registry is not None:
            docker_repo = '{0}/{1}'.format(registry, docker_repo)
            logger.info('Also tagging {0}'.format(docker_repo))
            client.tag(img_id, docker_repo, docker_tag)
        client.push(docker_repo)
    return img_id


class Summary(object):
    def __init__(self):
        self._summary = {}
        self._has_exc = False

    def _add_data(self, image, linestr, data):
        if image not in self._summary:
            self._summary[image] = { linestr: data }
        else:
            self._summary[image][linestr] = data

    def add_exception(self, image, line, exc):
        lineno, linestr = line
        self._add_data(image, linestr, { 'line': lineno, 'exc': str(exc) })
        self._has_exc = True

    def add_success(self, image, line, img_id):
        lineno, linestr = line
        self._add_data(image, linestr, { 'line': lineno, 'id': img_id })

    def print_summary(self, logger=None):
        linesep = ''.center(61, '-') + '\n'
        s = 'BREW BUILD SUMMARY\n' + linesep
        success = 'OVERALL SUCCESS: {}\n'.format(not self._has_exc)
        details = linesep
        for image, lines in self._summary.iteritems():
            details = details + '{}\n{}'.format(image, linesep)
            for linestr, data in lines.iteritems():
                details = details + '{0:2} | {1} | {2:50}\n'.format(
                    data['line'],
                    'KO' if 'exc' in data else 'OK',
                    data['exc'] if 'exc' in data else data['id']
                )
            details = details + linesep
        if logger:
            logger.info(s + success + details)
        else:
            print s, success, details