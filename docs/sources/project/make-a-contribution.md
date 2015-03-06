page_title: Make a project contribution
page_description: Basic workflow for Docker contributions
page_keywords: contribute, pull request, review, workflow, white-belt, black-belt, squash, commit

# Make a project contribution

Contributing is a process where you work with Docker maintainers and the community to improve Docker. There is a formal process for contributing. We try to keep our contribution process simple so you want to come back.


In this section, you will create a new branch and work on some Docker code that you choose. Before you work through this process, take a few minutes to read through the next section which explains our basic contribution workflow. 

## The basic contribution workflow

<<<<<<< HEAD
<<<<<<< HEAD
You are about to work through our basic contribution workflow by fixing a single *white-belt* issue in the `docker/docker` repository. The workflow for fixing simple issues looks like this:
=======
You about to work through our basic contribution workflow by fixing a single *white-belt* issue in the `docker/docker` repository. The workflow for fixing simple issues looks like this:
>>>>>>> 6f40419... Tweaking from last run thru
=======
You are about to work through our basic contribution workflow by fixing a single *white-belt* issue in the `docker/docker` repository. The workflow for fixing simple issues looks like this:
>>>>>>> 2834bbe... Last minut check pass; fixes

![Simple process](/project/images/existing_issue.png)

All Docker repositories have code and documentation. This workflow works for either content type. For example, you can find and fix doc or code issues. Also, you can propose a new Docker feature or propose a new Docker tutorial. 

Some workflow stages have slight differences for code or documentation contributions. When you reach that point in the flow, we make sure to tell you.

## Find and claim an existing issue

An existing issue is something reported by a Docker user. As issues come in, our maintainers triage them. Triage is its own topic. For now, it is important for you to know that triage includes ranking issues according to difficulty. 

Triaged issues have either a **white-belt** or **black-belt** label.   A **white-belt** issue is considered an easier issue. Issues can have more than one the **white-belt** label, for example, **bug**, **improvement**, **/project/doc**, and so forth. These other labels are their for filtering purposes but you might also find them helpful.

In the next procedure, you find and claim an open white-belt issue.

<<<<<<< HEAD
<<<<<<< HEAD
1. Go to the `docker/docker` <a
	href="https://github.com/docker/docker" target="_blank">repository</a>.
=======
1. Go to the `docker/docker` repository.
>>>>>>> 6f40419... Tweaking from last run thru
=======
1. Go to the `docker/docker` <a
	href="https://github.com/docker/docker" target="_blank">repository</a>.
>>>>>>> 2834bbe... Last minut check pass; fixes

2. Click on the "Issues" link.

   	A list of the open issues appears. 
		
	![Open issues](/project/images/issue_list.png)

3. Look for the **white-belt** items on the list.

4. Click on the "Labels" dropdown and select  **white-belt**.

	The system filters to show only open **white-belt** issues.

5. Open an issue that interests you.

	The comments on the issues can tell you both the problem and the potential 
	solution.
	
6. Make sure that no other user has chosen to work on the issue.

    We don't allow external contributors to assign issues to themselves, so you
    need to read the comments to find if a user claimed an issue by saying:
    
    * "I'd love to give this a try~"
    * "I'll work on this!"
    * "I'll take this."
    
    The community is very good about claiming issues explicitly.

7. When you find an open issue that both interests you and is unclaimed, claim it yourself by adding a comment.

	![Easy issue](/project/images/easy_issue.png)

	This example uses issue 11038. Your issue # will be different depending on
	what you claimed.
	
8. Make a note of the issue number; you'll need it later.

## Sync your fork and create a new branch

If you have followed along in this guide, you forked the `docker/docker` repository. Maybe that was an hour ago or a few days ago. In any case, before you start working on your issue, sync your repository with the upstream `docker/docker` master. Syncing ensures your repository has the latest changes.

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

		$ git push
		Username for 'https://github.com': moxiegirl
		Password for 'https://moxiegirl@github.com': 
		Counting objects: 223, done.
		Compressing objects: 100% (38/38), done.
		Writing objects: 100% (69/69), 8.76 KiB | 0 bytes/s, done.
		Total 69 (delta 53), reused 47 (delta 31)
		To https://github.com/moxiegirl/docker.git
		   8e107a9..5035fa1  master -> master

	If you check 

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
		
## Work on your issue

The work you do for your issue depends on the specific issue you picked. This section gives you a step-by-step workflow. Where appropriate, it provides command examples. However, this is a generalized workflow, depending on your issue you may you may repeat steps or even skip some. How much time it takes you depends on you --- you could spend days or 30 minutes of your time.

Follow this workflow as you work:

1. Review the appropriate style guide.

	If you are changing code, review the <a href="../coding-style"
	target="_blank">coding style guide</a>. Changing documentation? Review the
	<a href="../doc-style" target="_blank">documentation style guide</a>. 
	
2. Make changes in your feature branch.

	Your feature branch you created in the last section. Here you use the
	development container. If you are making a code change, you can mount your
	source into a development container and iterate that way. For documentation
	alone, you can work on your local host. 
	
	Review <a href="../set-up-dev-env" target="_blank">if you forgot the details
<<<<<<< HEAD
<<<<<<< HEAD
	of working with a container</a>.
=======
	of working with a container.</a>.
>>>>>>> 6f40419... Tweaking from last run thru
=======
	of working with a container</a>.
>>>>>>> 2834bbe... Last minut check pass; fixes

3. Test your changes as you work.

	If you have followed along with the guide, you know the `make test` target
	runs the entire test suite and `make docs` builds the documentation. If you
	forgot the other test targets, see the documentation for <a
	href="../test-and-docs" target="_blank">testing both code and
	documentation</a>.  
	
4. For code changes, add unit tests if appropriate.

	If you add new functionality or change existing functionality, you should
	add a unit test also. Use the existing test files for inspiration. Aren't
	sure if you need tests? Skip this step; you can add them later in the
	process if necessary.
	
5. Format your source files correctly.

	<style type="text/css">
	.tg  {border-collapse:collapse;border-spacing:0;margin-bottom:15px;}
	.tg td{font-family:Arial, sans-serif;font-size:14px;padding:5px 5px;border-style:solid;border-width:1px;overflow:hidden;word-break:normal;vertical-align:top;}
	.tg th{font-family:Arial, sans-serif;font-size:14px;font-weight:bold;padding:5px 5px;border-style:solid;border-width:1px;overflow:hidden;word-break:normal;text-align:left;}
	.tg .tg-e3zv{width:150px;}
	</style>
	<table class="tg">
	  <tr>
		<th class="tg-e3zv">File type</th>
		<th class="tg-031e">How to format</th>
	  </tr>
	  <tr>
		<td class="tg-e3zv"><code>.go</code></td>
		<td class="tg-031e"><p>Format <code>.go</code> files using the <code>gofmt</code> command. For example, if you edited the `docker.go` file you would format the file
		like this:
		</p>
		<p><code>$ gofmt -s -w file.go</code></p>
		<p>	
		Most file editors have a plugin to format for you. Check your editor's
		documentation.
		</p>
		</td>
	  </tr>
	  <tr>
		<td class="tg-e3zv"><code>.md</code> and non-<code>.go</code> files</td>
		<td class="tg-031e">Wrap lines to 80 characters.</td>
	  </tr>
	</table>
	

6. List your changes.

		$ git status
		On branch 11038-fix-rhel-link
		Changes not staged for commit:
		  (use "git add <file>..." to update what will be committed)
		  (use "git checkout -- <file>..." to discard changes in working directory)

			modified:   docs/sources/installation/mac.md
			modified:   docs/sources/installation/rhel.md
			
	The `status` command lists what changed in the repository. Make sure you see
	the changes you expect.
	
7. Add your change to Git.

		$ git add docs/sources/installation/mac.md
		$ git add docs/sources/installation/rhel.md
		
		
8. Commit your changes making sure you use the `-s` flag to sign your work.

		$ git commit -s -m "Fixing RHEL link"
	
9. Push your change to your repository.
		 
		$ git push --set-upstream origin 11038-fix-rhel-link
		Username for 'https://github.com': moxiegirl
		Password for 'https://moxiegirl@github.com': 
		Counting objects: 60, done.
		Compressing objects: 100% (7/7), done.
		Writing objects: 100% (7/7), 582 bytes | 0 bytes/s, done.
		Total 7 (delta 6), reused 0 (delta 0)
		To https://github.com/moxiegirl/docker.git
		 * [new branch]      11038-fix-rhel-link -> 11038-fix-rhel-link
		Branch 11038-fix-rhel-link set up to track remote branch 11038-fix-rhel-link from origin.
		
10. Open your fork on GitHub to see your change.

## Create a pull request to docker/docker

A pull request sends your code to the Docker maintainers for review. Your pull request goes from your forked repository to the `docker/docker` repository.  You can see <a href="https://github.com/docker/docker/pulls" target="_blank">the list of active pull requests to Docker</a> on GitHub. 

To create a pull request for your change:

1. In a terminal window, go to the root of your `docker-fork` repository. 

		$ cd ~/repos/docker-fork

2. Checkout your feature branch.

		$ git checkout 11038-fix-rhel-link
		Already on '11038-fix-rhel-link'

3. Run the full test suite on your branch.

		$ make test
		
	All the tests should pass. If they don't, find out why and correct the
	situation. If you also modified the documentation,  run `make docs` and
	check your work.
	
4.  Update your remote repository with any changes that result from your last minute checks.	

	Use the `git add`, the `git commit -s`, and `git push` commands to do this.
	
4. Fetch any of the last minute changes from `docker/docker`.

		$ git fetch upstream master
		From github.com:docker/docker
		 * branch            master     -> FETCH_HEAD

5. Squash your individual separate commits into one by using Git’s interactive rebase:

		$ git rebase -i upstream/master
		
	This commit will open up your favorite editor with all the comments from
	all your latest commits.
	
		pick 1a79f55 Tweak some of the other text for grammar
		pick 53e4983 Fix a link
		pick 3ce07bb Add a new line about RHEL
		
6. Replace the `pick` keyword with `squash` on all but the first commit.
	
		pick 1a79f55 Tweak some of the other text for grammar
		squash 53e4983 Fix a link
		squash 3ce07bb Add a new line about RHEL	
				
	After closing the file, `git` opens your editor again to edit the commit
	message. 
	
7. Save your commit message.


8. Push any changes to your fork on GitHub.

		$ git push origin 11038-fix-rhel-link
		
9. Open your browser to your fork on GitHub.

	You should see the latest activity from your branch.
	
	![Latest commits](/project/images/latest_commits.png)

	
10. Click "Compare & pull request".

	The system displays the pull request dialog. 
	
	![PR dialog](/project/images/to_from_pr.png)
	
	The pull request compares your changes to the `master` branch on the `docker/docker` repository.

11. Edit the dialog's description and add a reference to the issue you are fixing.

	GitHub helps you out by searching for the issue as you type.
	
	![Fixes issue](/project/images/fixes_num.png)

12. Scroll down and verify the PR contains the commits and changes you expect.

	For example, is the file count correct? Are the changes in the files what
	you expect.
	
	![Commits](/project/images/commits_expected.png)

13. Press "Create pull request".

	The system creates the request and opens it for you in the `docker/docker`
	repository.

	![Pull request made](/project/images/pull_request_made.png)

## Your pull request under review

At this point, your pull request is reviewed. The first reviewer is Gordon. He might who might look slow in this picture: 

![Gordon](/project/images/gordon.jpeg)

He is actually pretty fast over a network. He checks your pull request (PR) for common problems like missing signatures. If Gordon finds a problem, he'll send  an email to your GitHub user.

After Gordon, the core Docker maintainers look at your pull request and comment on it. The shortest comment you might see is `LGTM` which means **l**ooks-**g**ood-**t**o-**m**e. If you get an `LGTM`, that is a good thing, you passed that review. 

For complex changes, maintainers may ask you questions or ask you to change something about your submission. All maintainer comments on a PR go to the email address associated with your GitHub account. Any GitHub user who "participates" in a PR receives an email to.  Participating means creating or commenting on a PR.

Our maintainers are very experienced Docker users and open source contributors. So, they value your time and will try to work efficiently with you by keeping their comments specific and brief.  If they ask you to make a change, you'll need to update your pull request with additional changes.

To update your existing pull request:

1. Change one or more files in your local `docker-fork` repository.

2. Commit the change with the `git commit --amend` command.

		$ git commit --amend 
		
	Git opens an editor containing your last commit message.
		
3. Adjust your last comment to reflect this new change.

		Added a new sentence per Anaud's suggestion	

		Signed-off-by: Mary Anthony <mary@docker.com>

		# Please enter the commit message for your changes. Lines starting
		# with '#' will be ignored, and an empty message aborts the commit.
		# On branch 11038-fix-rhel-link
		# Your branch is up-to-date with 'origin/11038-fix-rhel-link'.
		#
		# Changes to be committed:
		#		modified:   docs/sources/installation/mac.md
		#		modified:   docs/sources/installation/rhel.md

4. Push to your origin.

		$ git push origin
		
5. Open your browser to your pull request on GitHub.

	You should see your pull request now contains your newly pushed code.

6. Add a comment to your pull request.
	
	GitHub only notifies PR participants when you comment. For example, you can
	mention that you updated your PR. Your comment alerts the maintainers that
	you made an update.
	
A change requires LGTMs from an absolute majority of an affected component's maintainers. For example, if you change `docs/` and `registry/` code, an absolute majority of the `docs/` and the `registry/` maintainers must approve your PR. Once you get approval, we merge your pull request into Docker's `master` cod branch. 

## After the merge

It can take time to see a merged pull request in Docker's official release. A master build is available almost immediately though. Docker builds and updates its development binaries after each merge to `master`. 

1. Browse to <a href="https://master.dockerproject.com/" target="_blank">https://master.dockerproject.com/</a>.

2. Look for the binary appropriate to your system.

3. Download and run the binary.

	You might want to run the binary in a container though. This
	will keep your local host environment clean.

4. View any documentation changes at <a href="http://docs.master.dockerproject.com/" target="_blank">docs.master.dockerproject.com</a>. 

Once you've verified everything merged, feel free to delete your feature branch from your fork. For information on how to do this, <a href="https://help.github.com/articles/deleting-unused-branches/" target="_blank">see the GitHub help on deleting branches</a>.  

## Where to go next

At this point, you have completed all the basic tasks in our contributors guide. If you enjoyed contributing, let us know by completing another **white-belt** issue or two. We really appreciate the help. 

If you are very experienced and want to make a major change, go onto [learn about advanced contributing](/project/advanced-contributing).