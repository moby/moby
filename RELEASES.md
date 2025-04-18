# 🚀 Releases

This document outlines the Moby project’s release process, including cadence, versioning, and prioritization of fixes.

## 📌 Versioning

Releases of moby will be versioned using dotted triples, similar to
[Semantic Version](http://semver.org/). For the purposes of this document, we
will refer to the respective components of this triple as
`<major>.<minor>.<patch>-[suffix[N]]`. The version number may have additional
information, such as alpha, beta and release candidate qualifications along with
a numeric identifier. Such releases will be considered "pre-releases".

> [!NOTE]
> v17.03-v20.10 releases were following [CalVer](https://calver.org/).

## ⏳ Release cadence

The project will release as quickly as possible while ensuring stability and
timely feature delivery. Maintainers curate a feature roadmap and set milestones
for each release.

Release dates assigned to milestones are commitments, and any changes will be
announced to the community via an
[announcement discussion](https://github.com/moby/moby/discussions/categories/announcements).
To meet deadlines, some features may be deferred to future releases.

If your issue or feature is missing from the roadmap, please open a GitHub issue
or comment to request its inclusion.

## 🎯 Major and minor releases

Major and minor releases of moby will be made from main. Releases will be marked
with GPG signed tags and posted at https://github.com/moby/moby/releases. The
tag will be of the format `v<major>.<minor>.<patch>-[suffix[N]]`.

After a minor release, a branch will be created, with the format
`release/<major>.<minor>` from the minor tag. All further patch releases will be
done from that branch. For example, after the release of `v42.0.0`, a branch
`release/42.0` would be created from that tag. All future patch releases will be
done against that branch.

## 🛠️ Patch releases

Patch releases are made directly from release branches and will be done as needed
by the release branch owners.

After each release a patch release milestone is created. The creation of a patch
release is is not an obligation to actually crate a patch release. It serves as
a collection point for issues and pull requests that can justify a patch
release.

> [!IMPORTANT] 
> Security releases are also "patch releases", but follow a
> different procedure. Security releases are developed in a private repository,
> released and tested under embargo before they become publicly available.

## 🔥 Prioritization of fixes

Bugs in the patch milestone will have an assigned priority from the table below.
See this project's pinned issue on project workflow for more information on
labels used to indicate priority.

| Priority | Description                                                                                                                       |
| -------- | --------------------------------------------------------------------------------------------------------------------------------- |
| P0       | Urgent: Security, critical bugs, blocking issues. P0 basically means drop everything you are doing until this issue is addressed. |
| P1       | Important: P1 issues are a top priority and a must-have for the next release. Patch releases should happen within a week.         |
| P2       | Normal priority: default priority applied.                                                                                        |

> [!NOTE]
> Note that only critical (P0) patches will trigger a patch release and
> the release will include all fixes to P0. Some P1/P2 issues _may_ be included
> as part of the patch release depending on the severity of the issues and the
> risk associated with patch.

## 🎬 Pre-releases

Pre-releases, such as alphas, betas and release candidates will be conducted
from their source branch. For major and minor releases, these releases will be
done from `master`. For patch releases, it is uncommon to have pre-releases but
they may have an `rc` based on the discretion of the release branch owners.
