---
name: behavior-specs
description: >-
  Write, extend, or review the behavior specifications of the Moby daemon: the Sphinx-Needs specs
  in docs/spec, for networking and any subsystem added later. Use when adding or changing an
  expectation, a requirement, principle, rationale, host requirement or convergence ceiling; when
  deciding whether a behavior is a contract, a defect, provisional or a detector; when a code
  change alters behavior that a spec states; when reviewing or adjudicating a spec change; or when
  starting a spec for a subsystem that has none.
---

# Behavior specs

The specs, and the conventions for working on them, are written for every contributor, human or
agent. This skill does not restate them. Read them before writing anything, and follow them:

- `docs/spec/src/contributing.rst`: what a code change that alters specified behavior needs, the
  steps of adding or changing an expectation, the wording conventions, how to establish what the
  daemon does, how to review a spec change, what goes outside the specs, starting a spec for
  another subsystem, and commit messages.
- `docs/spec/src/README.rst`: the model that every spec shares, including the rules that decide a
  category, a requirement's reach, `depends_on`, host requirements and the targets of a defect.
- `docs/spec/src/<subsystem>/README.rst`: the subsystem's own conventions. For networking,
  `docs/spec/src/network/README.rst`.
- `docs/spec/src/principles.rst`, `docs/spec/src/id-style-guide.rst` and `docs/spec/src/conf.py`.
- The Requirements and Rationales sections at the start of each spec document.

Edit only the reStructuredText sources in `docs/spec/src/`, then run
`make -C docs/spec fmt md validate`. If these documents disagree with this skill, they hold: fix
the skill in the same change.

## What applies to an agent in particular

- **Settle facts yourself.** Read the code, then run a throwaway daemon as `contributing.rst`
  says, before you ask the user about what the daemon does. Remove every container and volume that
  you start.
- **Decide what the rules settle.** A question that the README's rules, the principles and the
  reliance test answer once the facts are known is yours to decide. Ask only about a new product
  commitment, or a question that no rule reaches.
- **Amend a rule rather than patch its instances.** When findings of one class keep coming back
  because they conflict with a rule, propose the amendment to the README or `principles.rst`, with
  its reason, as `contributing.rst` describes.
- **Review in rounds with subagents.** For a large change, run rounds of two kinds of reviewer:
  scope reviewers that read the sources, the README and the code, and builder checkers that read
  only the generated Markdown, as a test author would, and report each expectation whose verdict
  they cannot decide or could decide two ways. Give builder checkers none of the reasoning behind
  the specs: a reviewer who knows it fills the gaps without noticing them. Adjudicate every finding,
  apply the fixes, and start another round until a round changes nothing. Then get an independent
  second read from a reviewer that took no part in the rounds, such as another model. If it changes
  anything, go back to the rounds.
- **Verify a subagent's finding before acting on it.** Check its claim against the code, and drop
  a finding that states a difference without a consequence.
- **Keep security private.** Never describe an unfixed vulnerability in a spec, a commit message,
  an issue or a pull request, even when the user's own notes do. Raise it with the user in the
  conversation.
