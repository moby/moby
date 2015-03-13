page_title: Make a project contribution
page_description: Basic workflow for Docker contributions
page_keywords: contribute, pull request, review, workflow, white-belt, black-belt, squash, commit

<!-- TODO (@thaJeztah) remove after docs/base is updated -->
<style type="text/css">
.tg    {border-collapse:collapse;border-spacing:0;margin-bottom:15px;}
.tg td {background-color: #fff;padding:5px 5px;border-style:solid;border-width:1px;overflow:hidden;word-break:normal;vertical-align:top;}
.tg th {font-weight:bold;padding:5px 5px;border-style:solid;border-width:1px;overflow:hidden;word-break:normal;text-align:left;}
.tg .tg-e3zv{width:150px;}
</style>

<style>

/* GitHub label styles */
.gh-label {
    display: inline-block;
    padding: 3px 4px;
    font-size: 11px;
    font-weight: bold;
    line-height: 1;
    color: #fff;
    border-radius: 2px;
    box-shadow: inset 0 -1px 0 rgba(0,0,0,0.12);
}

.gh-label.black-belt  { background-color: #000000; color: #ffffff; }
.gh-label.bug         { background-color: #fc2929; color: #ffffff; }
.gh-label.improvement { background-color: #bfe5bf; color: #2a332a; }
.gh-label.project-doc { background-color: #207de5; color: #ffffff; }
.gh-label.white-belt  { background-color: #ffffff; color: #333333; }

</style>


# Find and claim an issue

On this page, you choose what you want to work on. As a contributor you can work
on whatever you want. If you are new to contributing, you should start by
working with our known issues.

## Understand the issue types

An existing issue is something reported by a Docker user. As issues come in,
our maintainers triage them. Triage is its own topic. For now, it is important
for you to know that triage includes ranking issues according to difficulty. 

Triaged issues have either a <strong class="gh-label white-belt">white-belt</strong> 
or <strong class="gh-label black-belt">black-belt</strong> label.
A <strong class="gh-label white-belt">white-belt</strong> issue is considered
an easier issue. Issues can have more than one label, for example, 
<strong class="gh-label bug">bug</strong>, 
<strong class="gh-label improvement">improvement</strong>, 
<strong class="gh-label project-doc">project/doc</strong>, and so forth. 
These other labels are there for filtering purposes but you might also find
them helpful.


## Claim a white-belt issue

In this section, you find and claim an open white-belt issue.


1. Go to the `docker/docker` <a
	href="https://github.com/docker/docker" target="_blank">repository</a>.

2. Click on the "Issues" link.

    A list of the open issues appears. 

    ![Open issues](/project/images/issue_list.png)

3. Look for the <strong class="gh-label white-belt">white-belt</strong> items on the list.

4. Click on the "labels" dropdown and select  <strong class="gh-label white-belt">white-belt</strong>.

    The system filters to show only open <strong class="gh-label white-belt">white-belt</strong> issues.

5. Open an issue that interests you.

    The comments on the issues can tell you both the problem and the potential 
    solution.

6. Make sure that no other user has chosen to work on the issue.

    We don't allow external contributors to assign issues to themselves, so you
    need to read the comments to find if a user claimed an issue by saying:
    
    - "I'd love to give this a try~"
    - "I'll work on this!"
    - "I'll take this."
    
    The community is very good about claiming issues explicitly.

7. When you find an open issue that both interests you and is unclaimed, claim it yourself by adding a comment.

    ![Easy issue](/project/images/easy_issue.png)

    This example uses issue 11038. Your issue # will be different depending on
    what you claimed.

8. Make a note of the issue number; you'll need it later.

## Sync your fork and create a new branch

If you have followed along in this guide, you forked the `docker/docker`
repository. Maybe that was an hour ago or a few days ago. In any case, before
you start working on your issue, sync your repository with the upstream
`docker/docker` master. Syncing ensures your repository has the latest
changes.

To sync your repository:

1. Open a terminal on your local host.

2. Change directory to the `docker-fork` root.

        $ cd ~/repos/docker-fork

3. Checkout the master branch.

        $ git checkout master
        Switched to branch 'master'
        Your branch is up-to-date with 'origin/master'.

    Recall that `origin/master` is a branch on your remote GitHub repository.

4. Make sure you have the upstream remote `docker/docker` by listing them.

        $ git remote -v
        origin	https://github.com/moxiegirl/docker.git (fetch)
        origin	https://github.com/moxiegirl/docker.git (push)
        upstream	https://github.com/docker/docker.git (fetch)
        upstream	https://github.com/docker/docker.git (

    If the `upstream` is missing, add it.

        $ git remote add upstream https://github.com/docker/docker.git

5. Fetch all the changes from the `upstream/master` branch.

        $ git fetch upstream/master
        remote: Counting objects: 141, done.
        remote: Compressing objects: 100% (29/29), done.
        remote: Total 141 (delta 52), reused 46 (delta 46), pack-reused 66
        Receiving objects: 100% (141/141), 112.43 KiB | 0 bytes/s, done.
        Resolving deltas: 100% (79/79), done.
        From github.com:docker/docker
           9ffdf1e..01d09e4  docs       -> upstream/docs
           05ba127..ac2521b  master     -> upstream/master

    This command says get all the changes from the `master` branch belonging to
    the `upstream` remote.

7. Rebase your local master with the `upstream/master`.

        $ git rebase upstream/master
        First, rewinding head to replay your work on top of it...
        Fast-forwarded master to upstream/master.

    This command writes all the commits from the upstream branch into your local
    branch.

8.  Check the status of your local branch.

        $ git status
        On branch master
        Your branch is ahead of 'origin/master' by 38 commits.
          (use "git push" to publish your local commits)
        nothing to commit, working directory clean

    Your local repository now has any changes from the `upstream` remote.  You
    need to push the changes to your own remote fork which is `origin/master`.

9. Push the rebased master to `origin/master`.

        $ git push origin
        Username for 'https://github.com': moxiegirl
        Password for 'https://moxiegirl@github.com': 
        Counting objects: 223, done.
        Compressing objects: 100% (38/38), done.
        Writing objects: 100% (69/69), 8.76 KiB | 0 bytes/s, done.
        Total 69 (delta 53), reused 47 (delta 31)
        To https://github.com/moxiegirl/docker.git
           8e107a9..5035fa1  master -> master

9. Create a new feature branch to work on your issue.

    Your branch name should have the format `XXXX-descriptive` where `XXXX` is
    the issue number you are working on. For example:

        $ git checkout -b 11038-fix-rhel-link
        Switched to a new branch '11038-fix-rhel-link'

    Your branch should be up-to-date with the upstream/master. Why? Because you
    branched off a freshly synced master.  Let's check this anyway in the next
    step.

9. Rebase your branch from upstream/master.

        $ git rebase upstream/master
        Current branch 11038-fix-rhel-link is up to date.

    At this point, your local branch, your remote repository, and the Docker
    repository all have identical code. You are ready to make changesfor your
    issues.


## Where to go next

At this point, you know what you want to work on and you have a branch to do
your work in.  Go onto the next section to learn [how to work on your
changes](/project/work-issue/).
