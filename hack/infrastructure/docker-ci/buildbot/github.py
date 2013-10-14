# This file is part of Buildbot.  Buildbot is free software: you can
# redistribute it and/or modify it under the terms of the GNU General Public
# License as published by the Free Software Foundation, version 2.
#
# This program is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
# FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more
# details.
#
# You should have received a copy of the GNU General Public License along with
# this program; if not, write to the Free Software Foundation, Inc., 51
# Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
#
# Copyright Buildbot Team Members

#!/usr/bin/env python
"""
github_buildbot.py is based on git_buildbot.py

github_buildbot.py will determine the repository information from the JSON 
HTTP POST it receives from github.com and build the appropriate repository.
If your github repository is private, you must add a ssh key to the github
repository for the user who initiated the build on the buildslave.

"""

import re
import datetime
from twisted.python import log
import calendar

try:
    import json
    assert json
except ImportError:
    import simplejson as json

# python is silly about how it handles timezones
class fixedOffset(datetime.tzinfo):
    """
    fixed offset timezone
    """
    def __init__(self, minutes, hours, offsetSign = 1):
        self.minutes = int(minutes) * offsetSign
        self.hours   = int(hours)   * offsetSign
        self.offset  = datetime.timedelta(minutes = self.minutes,
                                         hours   = self.hours)

    def utcoffset(self, dt):
        return self.offset

    def dst(self, dt):
        return datetime.timedelta(0)
    
def convertTime(myTestTimestamp):
    #"1970-01-01T00:00:00+00:00"
    # Normalize myTestTimestamp
    if myTestTimestamp[-1] == 'Z':
        myTestTimestamp = myTestTimestamp[:-1] + '-00:00'
    matcher = re.compile(r'(\d\d\d\d)-(\d\d)-(\d\d)T(\d\d):(\d\d):(\d\d)([-+])(\d\d):(\d\d)')
    result  = matcher.match(myTestTimestamp)
    (year, month, day, hour, minute, second, offsetsign, houroffset, minoffset) = \
        result.groups()
    if offsetsign == '+':
        offsetsign = 1
    else:
        offsetsign = -1
    
    offsetTimezone = fixedOffset( minoffset, houroffset, offsetsign )
    myDatetime = datetime.datetime( int(year),
                                    int(month),
                                    int(day),
                                    int(hour),
                                    int(minute),
                                    int(second),
                                    0,
                                    offsetTimezone)
    return calendar.timegm( myDatetime.utctimetuple() )

def getChanges(request, options = None):
        """
        Reponds only to POST events and starts the build process
        
        :arguments:
            request
                the http request object
        """
        payload = json.loads(request.args['payload'][0])
        import urllib,datetime
        fname = str(datetime.datetime.now()).replace(' ','_').replace(':','-')[:19]
        open('github_{0}.json'.format(fname),'w').write(json.dumps(json.loads(urllib.unquote(request.args['payload'][0])), sort_keys = True, indent = 2))

        if 'pull_request' in payload:
            user = payload['pull_request']['user']['login']
            repo = payload['pull_request']['head']['repo']['name']
            repo_url = payload['pull_request']['head']['repo']['html_url']
        else:
            user = payload['repository']['owner']['name']
            repo = payload['repository']['name']
            repo_url = payload['repository']['url']
        project = request.args.get('project', None)
        if project:
            project = project[0]
        elif project is None:
            project = ''
        # This field is unused:
        #private = payload['repository']['private']
        changes = process_change(payload, user, repo, repo_url, project)
        log.msg("Received %s changes from github" % len(changes))
        return (changes, 'git')

def process_change(payload, user, repo, repo_url, project):
        """
        Consumes the JSON as a python object and actually starts the build.
        
        :arguments:
            payload
                Python Object that represents the JSON sent by GitHub Service
                Hook.
        """
        changes = []

        newrev = payload['after'] if 'after' in payload else payload['pull_request']['head']['sha']
        refname = payload['ref'] if 'ref' in payload else payload['pull_request']['head']['ref']

        # We only care about regular heads, i.e. branches
        match = re.match(r"^(refs\/heads\/|)([^/]+)$", refname)
        if not match:
            log.msg("Ignoring refname `%s': Not a branch" % refname)
            return []

        branch = match.groups()[1]
        if re.match(r"^0*$", newrev):
            log.msg("Branch `%s' deleted, ignoring" % branch)
            return []
        else: 
            if 'pull_request' in payload:
                if payload['action'] == 'closed':
                    log.msg("PR#{} closed, ignoring".format(payload['number']))
                    return []
                changes = [{
                    'category'   : 'github_pullrequest',
                    'who'        : '{0} - PR#{1}'.format(user,payload['number']),
                    'files'      : [],
                    'comments'   : payload['pull_request']['title'], 
                    'revision'   : newrev,
                    'when'       : convertTime(payload['pull_request']['updated_at']),
                    'branch'     : branch,
                    'revlink'    : '{0}/commit/{1}'.format(repo_url,newrev),
                    'repository' : repo_url,
                    'project'  : project  }] 
                return changes
            for commit in payload['commits']:
                files = []
                if 'added' in commit:
                    files.extend(commit['added'])
                if 'modified' in commit:
                    files.extend(commit['modified'])
                if 'removed' in commit:
                    files.extend(commit['removed'])
                when =  convertTime( commit['timestamp'])
                log.msg("New revision: %s" % commit['id'][:8])
                chdict = dict(
                    who      = commit['author']['name'] 
                                + " <" + commit['author']['email'] + ">",
                    files    = files,
                    comments = commit['message'], 
                    revision = commit['id'],
                    when     = when,
                    branch   = branch,
                    revlink  = commit['url'], 
                    repository = repo_url,
                    project  = project)
                changes.append(chdict) 
            return changes
